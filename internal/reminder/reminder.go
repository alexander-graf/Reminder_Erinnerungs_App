package reminder

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type ReminderService struct {
	db             *sql.DB
	window         fyne.Window
	ticker         *time.Ticker
	done           chan bool
	shownReminders map[int]bool
}

func NewReminderService(db *sql.DB, window fyne.Window) *ReminderService {
	return &ReminderService{
		db:             db,
		window:         window,
		done:           make(chan bool),
		shownReminders: make(map[int]bool),
	}
}

func (r *ReminderService) Start() {
	r.ticker = time.NewTicker(10 * time.Second)
	log.Println("ReminderService gestartet - prüft alle 10 Sekunden")

	go func() {
		resetTicker := time.NewTicker(24 * time.Hour)
		for {
			select {
			case <-resetTicker.C:
				r.resetShownReminders()
				log.Println("Liste der gezeigten Erinnerungen zurückgesetzt")
			case <-r.done:
				resetTicker.Stop()
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-r.ticker.C:
				r.checkAppointments()
			case <-r.done:
				r.ticker.Stop()
				log.Println("ReminderService beendet")
				return
			}
		}
	}()
}

func (r *ReminderService) Stop() {
	r.done <- true
}

func (r *ReminderService) checkAppointments() {
	// Aktuelle Zeit + 5 Minuten
	checkTime := time.Now().Add(5 * time.Minute)

	// Debug-Ausgabe der Prüfzeit
	log.Printf("Prüfe Termine für: %s", checkTime.Format("2006-01-02 15:04"))

	// SQL-Query mit Debug-Ausgabe
	query := `
        SELECT id, title, date, time, priority 
        FROM appointments 
        WHERE datetime(date || ' ' || time) = datetime(?, ?)`
	queryTime := checkTime.Format("2006-01-02")
	queryMinute := checkTime.Format("15:04")

	log.Printf("SQL Query: %s mit Parametern: date=%s, time=%s", query, queryTime, queryMinute)

	rows, err := r.db.Query(query, queryTime, queryMinute)
	if err != nil {
		log.Printf("Fehler bei der Terminabfrage: %v", err)
		return
	}
	defer rows.Close()

	// Debug-Ausgabe der gefundenen Termine
	var count int
	for rows.Next() {
		count++
		var id int
		var title, date string
		var timeStr sql.NullString
		var priority sql.NullInt64
		if err := rows.Scan(&id, &title, &date, &timeStr, &priority); err != nil {
			log.Printf("Fehler beim Scannen eines Termins: %v", err)
			continue
		}

		// Überprüfe, ob die Zeit gültig ist
		if !timeStr.Valid {
			log.Printf("Überspringe Termin ID=%d: Keine gültige Zeit", id)
			continue
		}

		if !r.shownReminders[id] {
			log.Printf("Zeige Erinnerung für Termin: ID=%d, Titel=%s, Datum=%s, Zeit=%s, Priorität=%v",
				id, title, date, timeStr.String, priority)
			r.shownReminders[id] = true
			r.showReminder(id, title, date, timeStr.String, priority)
		} else {
			log.Printf("Erinnerung für Termin ID=%d wurde bereits gezeigt", id)
		}
	}

	if count == 0 {
		log.Printf("Keine Termine in 5 Minuten gefunden")
	}
}

func (r *ReminderService) showReminder(id int, title, date, timeStr string, priority sql.NullInt64) {
	priorityStr := "Keine"
	if priority.Valid {
		priorityStr = fmt.Sprintf("%d", priority.Int64)
	}

	content := widget.NewForm(
		widget.NewFormItem("Titel", widget.NewLabel(title)),
		widget.NewFormItem("Datum", widget.NewLabel(date)),
		widget.NewFormItem("Zeit", widget.NewLabel(timeStr)),
		widget.NewFormItem("Priorität", widget.NewLabel(priorityStr)),
	)

	// Erstelle den Dialog zuerst
	var d dialog.Dialog

	// Container für Buttons
	buttons := container.NewHBox(
		widget.NewButton("5 Min verschieben", func() {
			r.postponeAppointment(id, 5)
			// Schließe den Dialog erst nach der Verschiebung
			if d != nil {
				d.Hide()
			}
		}),
		widget.NewButton("Neu planen", func() {
			r.rescheduleAppointment(id, title)
			if d != nil {
				d.Hide()
			}
		}),
		widget.NewButton("OK", func() {
			if d != nil {
				d.Hide()
			}
		}),
	)

	// Vertikaler Container für Content und Buttons
	vBox := container.NewVBox(
		widget.NewLabel("In 5 Minuten beginnt:"),
		content,
		buttons,
	)

	// Custom Dialog
	d = dialog.NewCustom(
		"Terminerinnerung!",
		"", // Kein Standard-Button
		vBox,
		r.window,
	)
	d.Resize(fyne.NewSize(400, 300))
	d.Show()
}

func (r *ReminderService) postponeAppointment(id int, minutes int) {
	// Erst die aktuellen Werte abrufen
	var date string
	var timeStr sql.NullString
	err := r.db.QueryRow("SELECT date, time FROM appointments WHERE id = ?", id).Scan(&date, &timeStr)
	if err != nil {
		log.Printf("Fehler beim Abrufen des Termins: %v", err)
		dialog.ShowError(err, r.window)
		return
	}

	if !timeStr.Valid {
		log.Printf("Keine gültige Zeit für Termin ID=%d gefunden", id)
		dialog.ShowError(fmt.Errorf("Keine gültige Zeit für diesen Termin"), r.window)
		return
	}

	// Parse das aktuelle Datum und Zeit
	dateTime, err := time.Parse("2006-01-02 15:04", date+" "+timeStr.String)
	if err != nil {
		log.Printf("Fehler beim Parsen von Datum/Zeit: %v", err)
		dialog.ShowError(err, r.window)
		return
	}

	// Addiere nur 1 Minute zu der ursprünglichen Zeit
	// (da wir bereits 5 Minuten vor dem Termin sind)
	newDateTime := dateTime.Add(time.Duration(1) * time.Minute)

	// Update mit den neuen Werten
	_, err = r.db.Exec(`
        UPDATE appointments 
        SET time = ?
        WHERE id = ?`,
		newDateTime.Format("15:04"),
		id)
	if err != nil {
		log.Printf("Fehler beim Verschieben des Termins: %v", err)
		dialog.ShowError(err, r.window)
		return
	}

	delete(r.shownReminders, id)
	log.Printf("Termin ID=%d um 1 Minute verschoben auf %s", id, newDateTime.Format("15:04"))

	dialog.ShowInformation("Termin verschoben",
		fmt.Sprintf("Der Termin wurde auf %s verschoben.", newDateTime.Format("15:04")),
		r.window)
}

func (r *ReminderService) rescheduleAppointment(id int, title string) {
	// Hier können wir die bestehende editAppointment Funktion wiederverwenden
	// oder eine neue Variante erstellen
	// ... Implementation ...
}

func (r *ReminderService) resetShownReminders() {
	r.shownReminders = make(map[int]bool)
}
