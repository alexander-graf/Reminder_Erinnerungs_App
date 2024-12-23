package reminder

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type ReminderService struct {
	db             *sql.DB
	window         fyne.Window
	stopChan       chan struct{}
	shownReminders map[int]bool
}

func NewReminderService(db *sql.DB, window fyne.Window) *ReminderService {
	return &ReminderService{
		db:             db,
		window:         window,
		shownReminders: make(map[int]bool),
	}
}

func (r *ReminderService) Start() {
	r.stopChan = make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				r.checkAppointments()
			case <-r.stopChan:
				return
			}
		}
	}()
}

func (r *ReminderService) Stop() {
	r.stopChan <- struct{}{}
}

func (r *ReminderService) checkAppointments() {
	now := time.Now()
	rows, err := r.db.Query(`
		SELECT id, title, date, time, priority 
		FROM appointments 
		WHERE date = ? 
		AND time IS NOT NULL`,
		now.Format("2006-01-02"))
	if err != nil {
		log.Printf("Fehler beim Abrufen der Termine: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var title string
		var date string
		var timeStr string
		var priority sql.NullInt64
		if err := rows.Scan(&id, &title, &date, &timeStr, &priority); err != nil {
			log.Printf("Fehler beim Scannen der Termine: %v", err)
			continue
		}

		// Parse appointment time
		appointmentTime, err := time.Parse("15:04", timeStr)
		if err != nil {
			log.Printf("Fehler beim Parsen der Zeit: %v", err)
			continue
		}

		// Convert to full timestamp for today
		appointmentDateTime := time.Date(
			now.Year(), now.Month(), now.Day(),
			appointmentTime.Hour(), appointmentTime.Minute(),
			0, 0, now.Location())

		// Calculate time difference
		diff := appointmentDateTime.Sub(now)
		diffMinutes := int(diff.Minutes())

		// Exakt zum Termin (0-1 Minute Differenz)
		if diffMinutes >= 0 && diffMinutes < 1 {
			// Formatiere die Zeit für die Anzeige
			formattedTime := appointmentTime.Format("15:04")
			formattedDate := now.Format("02.01.2006")
			notificationText := fmt.Sprintf("%s, %s, %s", title, formattedTime, formattedDate)
			go r.showZenityNotification(notificationText, "", priority)
		}
		// 5-Minuten-Vorwarnung
		if diffMinutes >= 4 && diffMinutes < 5 {
			go r.showReminder(id, title, date, timeStr, priority)
		}
	}
}

func (r *ReminderService) showZenityNotification(title string, timing string, priority sql.NullInt64) {
	priorityText := ""
	if priority.Valid {
		priorityText = fmt.Sprintf("\nPriorität: %d", priority.Int64)
	}

	message := fmt.Sprintf("%s\n%s%s", title, timing, priorityText)

	// Hole die aktuelle Umgebung
	env := os.Environ()

	// Versuche verschiedene DISPLAY Werte
	displays := []string{":0", ":0.0", ":1", ":1.0"}

	for _, display := range displays {
		cmd := exec.Command("zenity", "--info",
			"--title=Terminerinnerung!",
			"--text="+message,
			"--width=400",
			"--height=200")

		// Setze die komplette Umgebung inkl. DISPLAY
		cmd.Env = append(env, "DISPLAY="+display)

		if err := cmd.Run(); err == nil {
			// Wenn erfolgreich, breche die Schleife ab
			return
		}
	}

	// Fallback: Wenn Zenity nicht funktioniert, verwende notify-send
	fallbackCmd := exec.Command("notify-send",
		"--urgency=critical",
		"--app-name=Terminerinnerung",
		"Terminerinnerung",
		message)

	if err := fallbackCmd.Run(); err != nil {
		log.Printf("Fehler beim Anzeigen der Benachrichtigung: %v", err)
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

func (r *ReminderService) DeleteAllAppointments() error {
	// Dialog zur Bestätigung
	confirmDialog := dialog.NewConfirm(
		"Alle Termine löschen",
		"Möchten Sie wirklich alle Termine unwiderruflich löschen?",
		func(confirm bool) {
			if confirm {
				// Führe das Löschen durch
				_, err := r.db.Exec("DELETE FROM appointments")
				if err != nil {
					dialog.ShowError(fmt.Errorf("Fehler beim Löschen aller Termine: %v", err), r.window)
					return
				}

				// Zeige Bestätigung
				dialog.ShowInformation("Erfolg", "Alle Termine wurden gelöscht.", r.window)

				// Setze die shownReminders zurück
				r.resetShownReminders()
			}
		},
		r.window,
	)
	confirmDialog.SetDismissText("Abbrechen")
	confirmDialog.SetConfirmText("Ja, alle löschen")
	confirmDialog.Show()

	return nil
}
