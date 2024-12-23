package main

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
)

// Struktur für Termine
type Appointment struct {
	Title    string
	Time     string
	Priority *int // Priority als Zeiger auf int
}

// Struktur für Aufgaben
type Task struct {
	Title     string
	Completed bool
}

var (
	db                *sql.DB
	appointmentsTable *widget.Table
	tasksTable        *widget.Table
	appointmentsList  [][]string
	tasksList         [][]string
)

// Funktion zum Initialisieren der Datenbank
func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./reminder.db")
	if err != nil {
		log.Fatal(err)
	}

	// Tabellen erstellen, falls sie nicht existieren
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS appointments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		time TEXT,
		priority INTEGER
	);
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		completed BOOLEAN
	);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}
}

// Funktion zum Hinzufügen eines Termins
func addAppointment(myWindow fyne.Window) {
	titleEntry := widget.NewEntry()
	timeEntry := widget.NewEntry()
	// Wenn ins Zeitfeld geklickt wird, öffnen wir Zenity
	timeEntry.OnTapped = func() {
		cmd := exec.Command("zenity", "--calendar", "--date-format=%Y-%m-%d")
		output, err := cmd.Output()
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		// Setze das ausgewählte Datum in das Feld
		timeEntry.SetText(strings.TrimSpace(string(output)))
	}

	// Erstelle ComboBox für Priorität
	prioritySelect := widget.NewSelect([]string{"1", "2", "3"}, nil)
	prioritySelect.PlaceHolder = "Priorität wählen"

	dialog.ShowForm("Neuen Termin hinzufügen", "Hinzufügen", "Abbrechen", []*widget.FormItem{
		widget.NewFormItem("Titel", titleEntry),
		widget.NewFormItem("Datum", timeEntry),
		widget.NewFormItem("Priorität", prioritySelect),
	}, func(submitted bool) {
		if submitted {
			title := titleEntry.Text
			time := timeEntry.Text

			// Priorität aus ComboBox
			var priority *int
			if prioritySelect.Selected != "" {
				p, _ := strconv.Atoi(prioritySelect.Selected)
				priority = &p
			}

			// Speichern des Termins in der Datenbank
			_, err := db.Exec("INSERT INTO appointments (title, time, priority) VALUES (?, ?, ?)",
				title, time, priority)
			if err != nil {
				log.Printf("Fehler beim Speichern des Termins: %v", err)
				dialog.ShowInformation("Fehler", "Fehler beim Speichern des Termins: "+err.Error(), myWindow)
				return
			}
			dialog.ShowInformation("Termin hinzugefügt",
				fmt.Sprintf("Titel: %s\nDatum: %s\nPriorität: %v", title, time, priority),
				myWindow)
		}
	}, myWindow)
}

// Funktion zum Hinzufügen einer Aufgabe
func addTask(myWindow fyne.Window) {
	titleEntry := widget.NewEntry()

	dialog.ShowForm("Neue Aufgabe hinzufügen", "Hinzufügen", "Abbrechen", []*widget.FormItem{
		widget.NewFormItem("Titel", titleEntry),
	}, func(submitted bool) {
		if submitted {
			title := titleEntry.Text

			// Speichern der Aufgabe in der Datenbank
			_, err := db.Exec("INSERT INTO tasks (title, completed) VALUES (?, ?)", title, false)
			if err != nil {
				log.Printf("Fehler beim Speichern der Aufgabe: %v", err) // Debugging-Information
				dialog.ShowInformation("Fehler", "Fehler beim Speichern der Aufgabe: "+err.Error(), myWindow)
				return
			}
			dialog.ShowInformation("Aufgabe hinzugefügt", "Titel: "+title, myWindow)
		}
	}, myWindow)
}

// Funktion zum Anzeigen aller Termine in einem neuen Fenster
func showAppointments(myWindow fyne.Window, myApp fyne.App) {
	rows, err := db.Query("SELECT title, time, priority FROM appointments")
	if err != nil {
		log.Printf("Fehler beim Abrufen der Termine: %v", err) // Debugging-Information
		dialog.ShowInformation("Fehler", "Fehler beim Abrufen der Termine: "+err.Error(), myWindow)
		return
	}
	defer rows.Close()

	var appointments [][]string
	for rows.Next() {
		var title, time string
		var priority sql.NullInt64 // Verwende sql.NullInt64 für die Priority
		if err := rows.Scan(&title, &time, &priority); err != nil {
			log.Printf("Fehler beim Scannen der Termine: %v", err) // Debugging-Information
			dialog.ShowInformation("Fehler", "Fehler beim Scannen der Termine: "+err.Error(), myWindow)
			return
		}
		priorityValue := "Keine Priorität"
		if priority.Valid {
			priorityValue = fmt.Sprintf("%d", priority.Int64)
		}
		appointments = append(appointments, []string{title, time, priorityValue})
	}

	if len(appointments) == 0 {
		dialog.ShowInformation("Termine", "Keine Termine gefunden.", myWindow)
		return
	}

	appointmentsList = appointments
	appointmentsTable = widget.NewTable(
		func() (int, int) {
			return len(appointmentsList), 5
		},
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel(""))
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			if id.Col < 3 {
				cell.(*fyne.Container).Objects[0].(*widget.Label).SetText(appointmentsList[id.Row][id.Col])
			} else if id.Col == 3 {
				deleteBtn := widget.NewButton("Löschen", func() {
					deleteAppointment(appointmentsList[id.Row][0], myWindow)
				})
				cell.(*fyne.Container).Objects = []fyne.CanvasObject{deleteBtn}
			} else if id.Col == 4 {
				editBtn := widget.NewButton("Ändern", func() {
					editAppointment(appointmentsList[id.Row][0], appointmentsList[id.Row][1], appointmentsList[id.Row][2], myWindow)
				})
				cell.(*fyne.Container).Objects = []fyne.CanvasObject{editBtn}
			}
		},
	)

	appointmentsTable.SetColumnWidth(0, 200)
	appointmentsTable.SetColumnWidth(1, 100)
	appointmentsTable.SetColumnWidth(2, 80)
	appointmentsTable.SetColumnWidth(3, 80)
	appointmentsTable.SetColumnWidth(4, 80)

	scrollContainer := container.NewScroll(appointmentsTable)
	content := container.NewPadded(scrollContainer)

	d := dialog.NewCustom("Alle Termine", "Schließen", content, myWindow)
	d.Resize(fyne.NewSize(800, 400))
	d.Show()
}

// Funktion zum Anzeigen aller Aufgaben in einem neuen Fenster
func showTasks(myWindow fyne.Window, myApp fyne.App) {
	rows, err := db.Query("SELECT title, completed FROM tasks")
	if err != nil {
		log.Printf("Fehler beim Abrufen der Aufgaben: %v", err) // Debugging-Information
		dialog.ShowInformation("Fehler", "Fehler beim Abrufen der Aufgaben: "+err.Error(), myWindow)
		return
	}
	defer rows.Close()

	var tasks [][]string
	for rows.Next() {
		var title string
		var completed bool
		if err := rows.Scan(&title, &completed); err != nil {
			log.Printf("Fehler beim Scannen der Aufgaben: %v", err) // Debugging-Information
			dialog.ShowInformation("Fehler", "Fehler beim Scannen der Aufgaben: "+err.Error(), myWindow)
			return
		}
		status := "Nicht abgeschlossen"
		if completed {
			status = "Abgeschlossen"
		}
		tasks = append(tasks, []string{title, status})
	}

	if len(tasks) == 0 {
		dialog.ShowInformation("Aufgaben", "Keine Aufgaben gefunden.", myWindow)
		return
	}

	tasksList = tasks
	tasksTable = widget.NewTable(
		func() (int, int) {
			return len(tasksList), 4
		},
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel(""))
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			if id.Col < 2 {
				cell.(*fyne.Container).Objects[0].(*widget.Label).SetText(tasksList[id.Row][id.Col])
			} else if id.Col == 2 {
				deleteBtn := widget.NewButton("Löschen", func() {
					deleteTask(tasksList[id.Row][0], myWindow)
				})
				cell.(*fyne.Container).Objects = []fyne.CanvasObject{deleteBtn}
			} else if id.Col == 3 {
				editBtn := widget.NewButton("Ändern", func() {
					editTask(tasksList[id.Row][0], tasksList[id.Row][1], myWindow)
				})
				cell.(*fyne.Container).Objects = []fyne.CanvasObject{editBtn}
			}
		},
	)

	tasksTable.SetColumnWidth(0, 250)
	tasksTable.SetColumnWidth(1, 130)
	tasksTable.SetColumnWidth(2, 80)
	tasksTable.SetColumnWidth(3, 80)

	scrollContainer := container.NewScroll(tasksTable)
	content := container.NewPadded(scrollContainer)

	d := dialog.NewCustom("Alle Aufgaben", "Schließen", content, myWindow)
	d.Resize(fyne.NewSize(800, 400))
	d.Show()
}

// Termin löschen
func deleteAppointment(title string, myWindow fyne.Window) {
	dialog.ShowConfirm("Löschen bestätigen",
		"Möchten Sie diesen Termin wirklich löschen?",
		func(confirm bool) {
			if confirm {
				_, err := db.Exec("DELETE FROM appointments WHERE title = ?", title)
				if err != nil {
					dialog.ShowError(err, myWindow)
					return
				}
				// Aktualisiere die Tabelle
				refreshAppointmentsTable()
			}
		}, myWindow)
}

// Termin bearbeiten
func editAppointment(title, time, priority string, myWindow fyne.Window) {
	titleEntry := widget.NewEntry()
	titleEntry.SetText(title)
	timeEntry := widget.NewEntry()
	timeEntry.SetText(time)
	priorityEntry := widget.NewEntry()
	priorityEntry.SetText(priority)

	dialog.ShowForm("Termin bearbeiten", "Speichern", "Abbrechen",
		[]*widget.FormItem{
			widget.NewFormItem("Titel", titleEntry),
			widget.NewFormItem("Zeit", timeEntry),
			widget.NewFormItem("Priorität", priorityEntry),
		},
		func(submitted bool) {
			if submitted {
				// Priorität konvertieren
				var priorityInt *int
				if priorityEntry.Text != "Keine Priorität" {
					if p, err := strconv.Atoi(priorityEntry.Text); err == nil {
						priorityInt = &p
					}
				}

				_, err := db.Exec("UPDATE appointments SET title = ?, time = ?, priority = ? WHERE title = ?",
					titleEntry.Text, timeEntry.Text, priorityInt, title)
				if err != nil {
					dialog.ShowError(err, myWindow)
					return
				}
				// Aktualisiere die Tabelle
				refreshAppointmentsTable()
			}
		}, myWindow)
}

// Aufgabe löschen
func deleteTask(title string, myWindow fyne.Window) {
	dialog.ShowConfirm("Löschen bestätigen",
		"Möchten Sie diese Aufgabe wirklich löschen?",
		func(confirm bool) {
			if confirm {
				_, err := db.Exec("DELETE FROM tasks WHERE title = ?", title)
				if err != nil {
					dialog.ShowError(err, myWindow)
					return
				}
				// Aktualisiere die Tabelle
				refreshTasksTable()
			}
		}, myWindow)
}

// Aufgabe bearbeiten
func editTask(title, status string, myWindow fyne.Window) {
	titleEntry := widget.NewEntry()
	titleEntry.SetText(title)
	completed := status == "Abgeschlossen"
	completedCheck := widget.NewCheck("Abgeschlossen", nil)
	completedCheck.Checked = completed

	dialog.ShowForm("Aufgabe bearbeiten", "Speichern", "Abbrechen",
		[]*widget.FormItem{
			widget.NewFormItem("Titel", titleEntry),
			widget.NewFormItem("Status", completedCheck),
		},
		func(submitted bool) {
			if submitted {
				_, err := db.Exec("UPDATE tasks SET title = ?, completed = ? WHERE title = ?",
					titleEntry.Text, completedCheck.Checked, title)
				if err != nil {
					dialog.ShowError(err, myWindow)
					return
				}
				// Aktualisiere die Tabelle
				refreshTasksTable()
			}
		}, myWindow)
}

// Hilfsfunktionen zum Aktualisieren der Tabellen
func refreshAppointmentsTable() {
	rows, err := db.Query("SELECT title, time, priority FROM appointments")
	if err != nil {
		return
	}
	defer rows.Close()

	appointmentsList = appointmentsList[:0] // Liste leeren
	for rows.Next() {
		var title, time string
		var priority sql.NullInt64
		if err := rows.Scan(&title, &time, &priority); err != nil {
			continue
		}
		priorityValue := "Keine Priorität"
		if priority.Valid {
			priorityValue = fmt.Sprintf("%d", priority.Int64)
		}
		appointmentsList = append(appointmentsList, []string{title, time, priorityValue})
	}
	if appointmentsTable != nil {
		appointmentsTable.Refresh()
	}
}

func refreshTasksTable() {
	rows, err := db.Query("SELECT title, completed FROM tasks")
	if err != nil {
		return
	}
	defer rows.Close()

	tasksList = tasksList[:0] // Liste leeren
	for rows.Next() {
		var title string
		var completed bool
		if err := rows.Scan(&title, &completed); err != nil {
			continue
		}
		status := "Nicht abgeschlossen"
		if completed {
			status = "Abgeschlossen"
		}
		tasksList = append(tasksList, []string{title, status})
	}
	if tasksTable != nil {
		tasksTable.Refresh()
	}
}

func main() {
	initDB()
	defer db.Close()

	myApp := app.New()
	myWindow := myApp.NewWindow("Reminder App")
	myWindow.Resize(fyne.NewSize(600, 400))

	// Positioniere das Hauptfenster auf dem zweiten Monitor (x > 1920)
	myWindow.Show() // Fenster muss sichtbar sein, bevor wir es positionieren
	myWindow.Resize(fyne.NewSize(600, 400))
	x, y := 2000, 200 // x > 1920 für zweiten Monitor
	myWindow.Canvas().Content().Move(fyne.NewPos(float32(x), float32(y)))

	hello := widget.NewLabel("Hello Fyne!")
	content := container.New(layout.NewVBoxLayout(),
		hello,
		widget.NewButton("Neuen Termin hinzufügen", func() {
			addAppointment(myWindow)
		}),
		widget.NewButton("Neue Aufgabe hinzufügen", func() {
			addTask(myWindow)
		}),
		widget.NewButton("Alle Termine anzeigen", func() {
			showAppointments(myWindow, myApp)
		}),
		widget.NewButton("Alle Aufgaben anzeigen", func() {
			showTasks(myWindow, myApp)
		}),
	)
	leftAligned := container.New(layout.NewHBoxLayout(), content, layout.NewSpacer())

	myWindow.SetContent(leftAligned)
	myWindow.ShowAndRun()
}
