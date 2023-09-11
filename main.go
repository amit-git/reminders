package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const TimeFormat = "1-2-2006"

type Rem struct {
	desc string
	at   time.Time
}

var rems map[int]Rem
var curIndex int
var RemsFilePath string

func main() {
	rems = map[int]Rem{}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Can't find current user information. " + err.Error())
		os.Exit(-1)
	}

	tasksFolder := homeDir + "/rm"
	if _, err := os.Stat(tasksFolder); os.IsNotExist(err) {
		if err := os.Mkdir(tasksFolder, os.ModePerm); err != nil {
			fmt.Println("Error creating " + tasksFolder + " :: " + err.Error())
			os.Exit(-1)
		}
	}

	RemsFilePath = tasksFolder + "/current"
	fmt.Println("Current Reminders are stored in ", RemsFilePath)

	hasStaleReminders := loadReminders()
	if hasStaleReminders {
		// cleanup stale reminders
		saveAllReminders()
	}

	for {
		cmd := readCmd()
		processCmd(cmd)
	}
}

func processCmd(cmd string) {
	if strings.HasPrefix(cmd, "a ") {
		if err := addReminder(cmd); err == nil {
			saveAllReminders()
			fmt.Println("Saved.")
		} else {
			fmt.Println("Error in processing ", cmd, " :: ", err.Error())
		}
	}

	// View all reminders
	if cmd == "va" {
		viewReminders(func(t time.Time) bool {
			return true
		})
	}

	// today's reminders
	if cmd == "vt" {
		viewReminders(func(t time.Time) bool {
			return isEqual(time.Now(), t)
		})
	}

	if strings.HasPrefix(cmd, "vt+") {
		daysStr := strings.Replace(cmd, "vt+", "", 1)
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			fmt.Println("Invalid command format ", err.Error())
			return
		}
		thresholdDays := days + 1
		viewReminders(func(t time.Time) bool {
			duration := time.Duration(thresholdDays) * 24 * time.Hour
			tNext := time.Now().Add(duration)
			return isGreater(tNext, t)
		})
	}

	if cmd == "h" {
		showHelp()
	}

	if strings.HasPrefix(cmd, "r") {
		r, err := regexp.Compile("r(\\d+)")
		if err != nil {
			fmt.Println("Error in parsing regexp ", err)
			return
		}
		matches := r.FindStringSubmatch(cmd)
		idx, _ := strconv.Atoi(matches[1])
		removeReminder(idx)
	}

	if cmd == "q" {
		fmt.Println("Bye")
		os.Exit(0)
	}
}

func removeReminder(idx int) {
	fmt.Println("Reminder :: ", rems[idx].desc, " Removed.")
	delete(rems, idx)
	saveAllReminders()
}

func showHelp() {
	fmt.Println("")
	fmt.Println("a <txt>@<date> : New reminder on <(M)M-(D)D-YYYY>")
	fmt.Println("r <rem-id>: Delete a reminder")
	fmt.Println("vt : Today's reminders")
	fmt.Println("vt+n : Reminders in next <n> days")
	fmt.Println("va : All reminders")
	fmt.Println("q : Quit")
}

func viewReminders(cond func(t time.Time) bool) {
	fmt.Println("\nReminders .....................")
	fmt.Println()
	// TODO - sort based on reminder times
	remIdsSorted := getSortedReminderIds()
	for _, i := range remIdsSorted {
		r := rems[i]
		if cond(r.at) {
			fmt.Printf("%d. %s [%v]\n", i, r.desc, r.at.Format(TimeFormat))
		}
	}
}

func isEqual(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

// t1 > t2
func isGreater(t1, t2 time.Time) bool {
	if t1.Year() == t2.Year() && t1.Month() == t2.Month() {
		return t1.Day() > t2.Day()
	}

	if t1.Year() == t2.Year() {
		return t1.Month() > t2.Month()
	}
	return t1.Year() > t2.Year()
}

func readCmd() string {
	fmt.Printf("\n> ")
	reader := bufio.NewReader(os.Stdin)
	cmd, e := reader.ReadString('\n')
	if e != nil {
		fmt.Println("Error in reading command " + e.Error())
		os.Exit(-1)
	}
	return strings.Replace(cmd, "\n", "", -1)
}

func addReminder(remCmd string) error {
	reminder := strings.Replace(remCmd, "a ", "", 1)
	ds, ts, err := parseReminder(reminder)
	if err != nil {
		return err
	}

	rems[curIndex] = Rem{
		desc: ds,
		at:   ts,
	}
	curIndex = curIndex + 1
	return nil
}

func parseReminder(cmd string) (string, time.Time, error) {
	remParts := strings.Split(cmd, "@")
	if len(remParts) != 2 {
		return "", time.Now(), errors.New("Invalid command format")
	}
	remTime, err := time.Parse(TimeFormat, remParts[1])
	if err != nil {
		return "", time.Now(), errors.New("Invalid command format")
	}

	return remParts[0], remTime, nil
}

func getFile(fileName string) *os.File {
	filePtr, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_SYNC, 0664)
	if err != nil {
		fmt.Printf("Unable to open %s :: %s", fileName, err.Error())
		os.Exit(-1)
	}
	return filePtr
}

func saveAllReminders() {
	f := getFile(RemsFilePath)
	defer f.Close()

	var lines string
	for _, rem := range rems {
		lines = lines + fmt.Sprintf("%s\t%s\n", rem.desc, rem.at.Format(TimeFormat))
	}

	_, err := f.Write([]byte(lines))
	if err != nil {
		fmt.Println("Error writing file system for current tasks file " + err.Error())
		os.Exit(-1)
	}
}

func loadReminders() bool {
	f := getFile(RemsFilePath)
	fileScanner := bufio.NewScanner(f)
	expiredRemindersFound := false
	for fileScanner.Scan() {
		line := fileScanner.Text()
		lineParts := strings.Split(line, "\t")
		if len(lineParts) == 2 {
			atValue, err := time.Parse(TimeFormat, lineParts[1])
			if err == nil {
				if !isGreater(time.Now(), atValue) {
					rems[curIndex] = Rem{
						desc: lineParts[0],
						at:   atValue,
					}
					curIndex = curIndex + 1
				} else {
					expiredRemindersFound = true
				}
			} else {
				fmt.Println("Error loading reminder date ", lineParts[1], " :: ", err.Error())
			}
		}
	}
	fmt.Printf("Loaded current reminders :: %d\n", len(rems))
	return expiredRemindersFound
}

func getSortedReminderIds() []int {
	remIds := make([]int, 0, len(rems))
	for k := range rems {
		remIds = append(remIds, k)
	}

	sort.Ints(remIds)
	return remIds
}
