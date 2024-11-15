package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Commit struct {
	Branch string
	Commit string
	Author string
	Date   time.Time
}

type Repo struct {
	Path    string
	Commits []Commit
}

type Day struct {
	Weekend  int
	NoOffice int
	Holiday  int
}

var (
	location                         *time.Location
	holidayStartTime, holidayEndTime time.Time
)

func main() {
	var err error

	location, err = time.LoadLocation("Europe/Madrid") //put here your time zone
	if err != nil {
		panic(err)
	}

	holidayStartTime = time.Date(2024, time.July, 15, 0, 0, 0, 0, location)
	holidayEndTime = time.Date(2024, time.August, 2, 23, 59, 59, 0, location)

	rootDir := "../" // Change this route to yours
	repositories := []string{}

	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Verifies if directory contains a .git one
		if info.IsDir() && strings.HasSuffix(info.Name(), ".git") {
			repositories = append(repositories, path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Failed going trough directories: %v", err)
	}

	rr := make([]Repo, 0)
	for _, repo := range repositories {
		fmt.Println(repo)
		rr = append(rr, *processRepository(repo))

	}

	checkDates("alopez@vottun.com", rr) //the username you want to check for dates
	os.Exit(1)
}

func checkDates(username string, rr []Repo) {
	users := make(map[string]byte)
	for _, r := range rr {
		t := make(map[string]Day)
		for _, c := range r.Commits {
			users[c.Author] = 0x00
			if c.Author != username {
				continue
			}
			localTime := c.Date.In(location)
			if localTime.Year() != 2024 {
				continue
			}
			date := localTime.Format("2006-01-02")

			if localTime.After(holidayStartTime) && localTime.Before(holidayEndTime) {
				if _, ok := t[date]; !ok {
					d := Day{Holiday: 1}
					t[date] = d
				} else {
					d := t[date]
					d.Holiday++
					t[date] = d
				}
			} else if localTime.Weekday() == time.Saturday || localTime.Weekday() == time.Sunday {
				if _, ok := t[date]; !ok {
					d := Day{Weekend: 1}
					t[date] = d
				} else {
					d := t[date]
					d.Weekend++
					t[date] = d
				}
			} else if localTime.Hour() >= 19 || localTime.Hour() <= 9 {
				if _, ok := t[date]; !ok {
					d := Day{NoOffice: 1}
					t[date] = d
				} else {
					d := t[date]
					d.NoOffice++
					t[date] = d
				}
			}
		}
		fmt.Println(r.Path)
		for k, v := range t {
			fmt.Printf("%s: %+v\n", k, v)
		}
		createInsert(r.Path, t)
	}
	for k, _ := range users {
		fmt.Println(k)
	}
}

func createInsert(path string, t map[string]Day) {
	sql := strings.Builder{}
	sql.WriteString("INSERT INTO app_working_days(repository, working_date, weekend, after_hours, holidays) VALUES\n")
	for k, v := range t {
		sql.WriteString(fmt.Sprintf("('%s', '%s', %d, %d, %d),\n", path[strings.LastIndex(path, "/")+1:], k, v.Weekend, v.NoOffice, v.Holiday))
	}

	sqlStr := sql.String()[0:sql.Len()-2] + ";"

	writeToFile(path, sqlStr)
}

func writeToFile(path, value string) error {

	dirPath := "./sql"

	createSqlDirectory(dirPath)
	filename := path[strings.LastIndex(path, "/")+1:]
	os.Remove(fmt.Sprintf("%s/%s.sql", dirPath, filename))
	f, err := os.OpenFile(fmt.Sprintf("./sql/%s.sql", filename),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(value); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func createSqlDirectory(dirPath string) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		// Create the directory
		err := os.Mkdir(dirPath, 0755) // 0755 is the permission (rwxr-xr-x)
		if err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}
}
func processRepository(repository string) *Repo {

	r := &Repo{Path: repository, Commits: make([]Commit, 0)}

	// Opens git repository (local)
	repo, err := git.PlainOpen(repository)
	if err != nil {
		log.Fatalf("failed opening repository: %v", err)
	}

	// it obtains all references (branches)
	refs, err := repo.References()
	if err != nil {
		log.Fatalf("failed getting references: %v", err)
	}

	// iterates on each branch
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		// Solo procesa si es un branch
		if ref.Name().IsBranch() {

			// obtains commits iterator for each branch
			commitIter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
			if err != nil {
				return err
			}

			// interates on commits
			err = commitIter.ForEach(func(c *object.Commit) error {
				cm := Commit{Branch: string(ref.Name()), Commit: c.Hash.String(), Author: c.Author.Email, Date: c.Author.When}
				r.Commits = append(r.Commits, cm)

				return nil
			})

			if err != nil {
				return nil
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("failed iterating branches: %v", err)
	}

	return r
}
