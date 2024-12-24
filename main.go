package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"Reminder_Erinnerungs_App/internal/reminder"

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

// Benutzerdefinierter Entry für Datumsauswahl
type DateEntry struct {
	widget.Entry
	window       fyne.Window
	justSelected bool // Flag für die Datumsauswahl
}

func NewDateEntry(window fyne.Window) *DateEntry {
	entry := &DateEntry{window: window}
	entry.ExtendBaseWidget(entry)
	entry.SetText("Klicken für Datumsauswahl")
	return entry
}

func (e *DateEntry) FocusGained() {
	if e.justSelected {
		e.justSelected = false
		e.Entry.FocusGained()
		return
	}

	// Prüfe ob Zenity installiert ist
	if _, err := exec.LookPath("zenity"); err != nil {
		dialog.ShowError(fmt.Errorf("Zenity ist nicht installiert. Bitte installieren Sie es mit 'sudo apt-get install zenity'"), e.window)
		return
	}

	cmd := exec.Command("zenity", "--calendar", "--date-format=%Y-%m-%d")
	output, err := cmd.CombinedOutput()

	// Ignoriere bestimmte Fehler (exit status 1 bei Abbruch)
	if err != nil && !strings.Contains(err.Error(), "exit status 1") {
		dialog.ShowError(fmt.Errorf("Fehler beim Ausführen von Zenity"), e.window)
		return
	}

	// Extrahiere nur das Datum aus der Ausgabe
	outputStr := string(output)
	// Suche nach einem Datum im Format YYYY-MM-DD
	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	if match := datePattern.FindString(outputStr); match != "" {
		e.justSelected = true
		e.SetText(convertToGermanDate(match))
	}

	e.Entry.FocusGained()
}

// Benutzerdefinierter Entry für Zeitauswahl
type TimeEntry struct {
	widget.Entry
	window       fyne.Window
	justSelected bool
}

func NewTimeEntry(window fyne.Window) *TimeEntry {
	entry := &TimeEntry{window: window}
	entry.ExtendBaseWidget(entry)
	entry.SetText("Klicken für Zeitauswahl")
	return entry
}

func (e *TimeEntry) FocusGained() {
	if e.justSelected {
		e.justSelected = false
		e.Entry.FocusGained()
		return
	}

	// Prüfe ob YAD installiert ist
	if _, err := exec.LookPath("yad"); err != nil {
		dialog.ShowError(fmt.Errorf("YAD ist nicht installiert. Bitte installieren Sie es mit 'sudo apt-get install yad'"), e.window)
		return
	}

	// Aktuelle Zeit ermitteln
	now := time.Now()
	currentHour := now.Hour()
	currentMinute := now.Minute()

	// Erstelle die Stunden- und Minutenlisten
	hours := make([]string, 24)
	for i := 0; i < 24; i++ {
		hours[i] = fmt.Sprintf("%02d", i)
	}
	minutes := make([]string, 60)
	for i := 0; i < 60; i++ {
		minutes[i] = fmt.Sprintf("%02d", i)
	}

	// Erstelle die Kommandozeile für YAD
	hoursStr := strings.Join(hours, "!")
	minutesStr := strings.Join(minutes, "!")

	cmd := exec.Command("yad", "--title=Zeit auswählen",
		"--form",
		"--field=Stunde:CB", hoursStr,
		"--field=Minute:CB", minutesStr,
		"--button=Auswählen:0",
		"--button=gtk-cancel:1",
		"--width=300",
		"--height=150",
		fmt.Sprintf("--entry-text=%02d", currentHour),
		fmt.Sprintf("--entry-text=%02d", currentMinute))

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Wenn der Benutzer abbricht (exit status 1), behalte den vorherigen Wert bei
		if strings.Contains(err.Error(), "exit status 1") {
			e.justSelected = true
			return
		}
		// Bei anderen Fehlern zeige eine Fehlermeldung
		errMsg := fmt.Sprintf("Fehler beim Ausführen von YAD: %v\nOutput: %s", err, string(output))
		dialog.ShowError(fmt.Errorf(errMsg), e.window)
		return
	}

	// Verarbeite die Ausgabe
	result := strings.TrimSpace(string(output))
	parts := strings.Split(result, "|")
	if len(parts) >= 2 {
		hour := strings.TrimSpace(parts[0])
		minute := strings.TrimSpace(parts[1])
		timeStr := fmt.Sprintf("%s:%s", hour, minute)
		e.justSelected = true
		e.SetText(timeStr)
	}

	e.Entry.FocusGained()
}

var (
	db                *sql.DB
	appointmentsTable *widget.Table
	tasksTable        *widget.Table
	appointmentsList  [][]string
	tasksList         [][]string
	reminderService   *reminder.ReminderService
)

// Neue Hilfsfunktionen für die Datumskonvertierung
func convertToGermanDate(isoDate string) string {
	// Konvertiert von YYYY-MM-DD zu DD.MM.YYYY
	if len(isoDate) != 10 {
		return isoDate
	}
	parts := strings.Split(isoDate, "-")
	if len(parts) != 3 {
		return isoDate
	}
	return fmt.Sprintf("%s.%s.%s", parts[2], parts[1], parts[0])
}

func convertToISODate(germanDate string) string {
	// Konvertiert von DD.MM.YYYY zu YYYY-MM-DD
	if len(germanDate) != 10 {
		return germanDate
	}
	parts := strings.Split(germanDate, ".")
	if len(parts) != 3 {
		return germanDate
	}
	return fmt.Sprintf("%s-%s-%s", parts[2], parts[1], parts[0])
}

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
		date TEXT,  -- Separates Datumsfeld
		time TEXT,  -- Separates Zeitfeld
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
	dateEntry := NewDateEntry(myWindow)
	timeEntry := NewTimeEntry(myWindow)

	// Aktuelles Datum im ISO-Format
	now := time.Now()
	dateEntry.SetText(convertToGermanDate(now.Format("2006-01-02")))

	// Aktuelle Zeit (gerundet auf die nächste Viertelstunde)
	roundedMinutes := ((now.Minute() + 14) / 15) * 15
	roundedTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), roundedMinutes, 0, 0, now.Location())
	timeEntry.SetText(roundedTime.Format("15:04"))

	// Erstelle ComboBox für Priorität mit Vorauswahl 1
	prioritySelect := widget.NewSelect([]string{"1", "2", "3"}, nil)
	prioritySelect.SetSelected("1") // Setze Priorität 1 als Standard
	prioritySelect.PlaceHolder = "Priorität wählen"

	dialog.ShowForm("Neuen Termin hinzufügen", "Hinzufügen", "Abbrechen", []*widget.FormItem{
		widget.NewFormItem("Titel", titleEntry),
		widget.NewFormItem("Datum", dateEntry),
		widget.NewFormItem("Uhrzeit", timeEntry),
		widget.NewFormItem("Priorität", prioritySelect),
	}, func(submitted bool) {
		if submitted {
			title := titleEntry.Text
			date := convertToISODate(dateEntry.Text) // Konvertiere zurück zu ISO für DB
			time := timeEntry.Text

			// Priorität aus ComboBox
			var priority *int
			if prioritySelect.Selected != "" {
				p, _ := strconv.Atoi(prioritySelect.Selected)
				priority = &p
			}

			// Speichern des Termins in der Datenbank
			_, err := db.Exec("INSERT INTO appointments (title, date, time, priority) VALUES (?, ?, ?, ?)",
				title, date, time, priority)
			if err != nil {
				log.Printf("Fehler beim Speichern des Termins: %v", err)
				dialog.ShowInformation("Fehler", "Fehler beim Speichern des Termins: "+err.Error(), myWindow)
				return
			}
			dialog.ShowInformation("Termin hinzugefügt",
				fmt.Sprintf("Titel: %s\nDatum: %s\nUhrzeit: %s\nPriorität: %s",
					title,
					dateEntry.Text, // Zeigt deutsches Format
					timeEntry.Text,
					func() string {
						if priority == nil {
							return "Keine"
						}
						return fmt.Sprintf("%d", *priority)
					}()),
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
	rows, err := db.Query("SELECT title, date, time, priority FROM appointments")
	if err != nil {
		log.Printf("Fehler beim Abrufen der Termine: %v", err)
		dialog.ShowInformation("Fehler", "Fehler beim Abrufen der Termine: "+err.Error(), myWindow)
		return
	}
	defer rows.Close()

	var appointments [][]string
	for rows.Next() {
		var title, date string
		var timeStr sql.NullString
		var priority sql.NullInt64
		if err := rows.Scan(&title, &date, &timeStr, &priority); err != nil {
			log.Printf("Fehler beim Scannen der Termine: %v", err)
			dialog.ShowInformation("Fehler", "Fehler beim Scannen der Termine: "+err.Error(), myWindow)
			return
		}

		// Setze einen Standardwert für die Zeit, wenn sie NULL ist
		time := "Keine Zeit"
		if timeStr.Valid {
			time = timeStr.String
		}

		priorityValue := "Keine Priorität"
		if priority.Valid {
			priorityValue = fmt.Sprintf("%d", priority.Int64)
		}

		// Konvertiere das Datum ins deutsche Format für die Anzeige
		germanDate := convertToGermanDate(date)
		appointments = append(appointments, []string{title, germanDate, time, priorityValue})
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
			// Erstelle einen Container mit einem Label für Text-Spalten und einem Button für Aktions-Spalten
			return container.NewHBox(
				widget.NewLabel(""),
				widget.NewButton("", nil), // Platzhalter für Buttons
			)
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			container := cell.(*fyne.Container)
			label := container.Objects[0].(*widget.Label)
			button := container.Objects[1].(*widget.Button)

			// Standardmäßig alles ausblenden
			label.Hide()
			button.Hide()

			if id.Col < 3 {
				// Text-Spalten (Titel, Zeit, Priorität)
				label.Show()
				label.SetText(appointmentsList[id.Row][id.Col])
			} else if id.Col == 3 {
				// Löschen-Button
				button.Show()
				button.SetText("Löschen")
				button.OnTapped = func() {
					deleteAppointment(appointmentsList[id.Row][0], myWindow)
				}
			} else if id.Col == 4 {
				// Ändern-Button
				button.Show()
				button.SetText("Ändern")
				button.OnTapped = func() {
					editAppointment(appointmentsList[id.Row][0], appointmentsList[id.Row][1], appointmentsList[id.Row][2], myWindow)
				}
			}
		},
	)

	appointmentsTable.SetColumnWidth(0, 200)
	appointmentsTable.SetColumnWidth(1, 100)
	appointmentsTable.SetColumnWidth(2, 80)
	appointmentsTable.SetColumnWidth(3, 80)
	appointmentsTable.SetColumnWidth(4, 80)

	scrollContainer := container.NewScroll(appointmentsTable)

	// Erstelle den "Alle Termine löschen" Button
	deleteAllButton := widget.NewButton("Alle Termine löschen", func() {
		reminderService.DeleteAllAppointments()
		refreshAppointmentsTable()
	})
	deleteAllButton.Importance = widget.DangerImportance

	// Erstelle einen horizontalen Container für den Button (rechts ausgerichtet)
	buttonContainer := container.NewHBox(
		layout.NewSpacer(), // Drückt den Button nach rechts
		deleteAllButton,
	)

	// Hauptcontainer mit Button oben und Tabelle darunter
	content := container.NewBorder(
		buttonContainer, // Top
		nil,             // Bottom
		nil,             // Left
		nil,             // Right
		scrollContainer, // Center (nimmt den restlichen Platz ein)
	)

	content = container.NewPadded(content)

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

	// Verwende die benutzerdefinierten Entries für Datum und Zeit
	dateEntry := NewDateEntry(myWindow)
	timeEntry := NewTimeEntry(myWindow)

	// Wenn time ein Datum enthält, parse es und setze die Werte
	if len(time) > 0 {
		dateEntry.SetText(time) // Zeit wird im deutschen Format angezeigt
		if t := strings.Split(time, " "); len(t) > 1 {
			timeEntry.SetText(t[1])
		}
	}

	// Erstelle ComboBox für Priorität
	prioritySelect := widget.NewSelect([]string{"1", "2", "3"}, nil)
	if priority != "Keine Priorität" {
		prioritySelect.SetSelected(priority)
	}
	prioritySelect.PlaceHolder = "Priorität wählen"

	dialog.ShowForm("Termin bearbeiten", "Speichern", "Abbrechen",
		[]*widget.FormItem{
			widget.NewFormItem("Titel", titleEntry),
			widget.NewFormItem("Datum", dateEntry),
			widget.NewFormItem("Uhrzeit", timeEntry),
			widget.NewFormItem("Priorität", prioritySelect),
		},
		func(submitted bool) {
			if submitted {
				// Priorität aus ComboBox
				var priorityInt *int
				if prioritySelect.Selected != "" {
					p, _ := strconv.Atoi(prioritySelect.Selected)
					priorityInt = &p
				}

				// Konvertiere das Datum zurück ins ISO-Format für die DB
				date := convertToISODate(dateEntry.Text)

				_, err := db.Exec(`
					UPDATE appointments 
					SET title = ?, date = ?, time = ?, priority = ? 
					WHERE title = ?`,
					titleEntry.Text, date, timeEntry.Text, priorityInt, title)
				if err != nil {
					dialog.ShowError(err, myWindow)
					return
				}

				// Zeige Bestätigung
				dialog.ShowInformation("Termin aktualisiert",
					fmt.Sprintf("Titel: %s\nDatum: %s\nUhrzeit: %s\nPriorität: %s",
						titleEntry.Text,
						dateEntry.Text,
						timeEntry.Text,
						func() string {
							if priorityInt == nil {
								return "Keine"
							}
							return fmt.Sprintf("%d", *priorityInt)
						}()),
					myWindow)

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
	rows, err := db.Query("SELECT title, date, time, priority FROM appointments")
	if err != nil {
		return
	}
	defer rows.Close()

	appointmentsList = appointmentsList[:0] // Liste leeren
	for rows.Next() {
		var title, date string
		var timeStr sql.NullString
		var priority sql.NullInt64
		if err := rows.Scan(&title, &date, &timeStr, &priority); err != nil {
			continue
		}

		// Setze einen Standardwert für die Zeit, wenn sie NULL ist
		time := "Keine Zeit"
		if timeStr.Valid {
			time = timeStr.String
		}

		priorityValue := "Keine Priorität"
		if priority.Valid {
			priorityValue = fmt.Sprintf("%d", priority.Int64)
		}

		// Konvertiere das Datum ins deutsche Format für die Anzeige
		germanDate := convertToGermanDate(date)
		appointmentsList = append(appointmentsList, []string{title, germanDate, time, priorityValue})
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
	// Unterdrücke Mesa-Fehlermeldungen
	os.Setenv("MESA_DEBUG", "silent")

	initDB()
	defer db.Close()

	myApp := app.New()
	myWindow := myApp.NewWindow("Reminder App")
	myWindow.Resize(fyne.NewSize(600, 400))

	// Reminder Service nach der Fenster-Erstellung initialisieren
	reminderService = reminder.NewReminderService(db, myWindow)
	reminderService.Start()
	defer reminderService.Stop()

	// Positioniere das Hauptfenster auf dem zweiten Monitor (x > 1920)
	myWindow.Show() // Fenster muss sichtbar sein, bevor wir es positionieren
	myWindow.Resize(fyne.NewSize(600, 400))
	x, y := 2000, 200 // x > 1920 für zweiten Monitor
	myWindow.Canvas().Content().Move(fyne.NewPos(float32(x), float32(y)))

	hello := widget.NewLabel("Reminder - Erinnerungs - App!")
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
