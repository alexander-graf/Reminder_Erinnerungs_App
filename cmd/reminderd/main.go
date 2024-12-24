package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"

	"Reminder_Erinnerungs_App/internal/reminder"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initialisiert die Datenbank und erstellt die benötigten Tabellen
func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Tabellen erstellen, falls sie nicht existieren
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS appointments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		date TEXT,
		time TEXT,
		priority INTEGER
	);
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		completed BOOLEAN
	);
	`
	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, err
	}

	return db, nil
}

func main() {
	// Verwende den korrekten Pfad zur Datenbank
	dbPath := "/home/alex/PycharmProjects/Reminder_Erinnerungs_App/reminder.db"

	// Initialisiere die Datenbank mit Tabellen
	db, err := initDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Erstelle einen minimalen ReminderService ohne GUI-Fenster
	reminderService := reminder.NewReminderService(db, nil)
	reminderService.Start()
	defer reminderService.Stop()

	// Warte auf Beendigungssignal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Reminder-Daemon gestartet. Datenbank: %s", dbPath)

	// Blockiere, bis ein Signal empfangen wird
	<-sigChan
	log.Println("Beende Reminder-Daemon...")
}
