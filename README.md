# Reminder-Erinnerungs-App

Eine Desktop-Anwendung zur Verwaltung von Terminen und Aufgaben, entwickelt in Go mit der Fyne GUI-Bibliothek.

## Funktionen

- Termine erstellen und verwalten mit:
  - Titel
  - Datum
  - Uhrzeit 
  - Priorität
- Aufgaben erstellen und verwalten mit:
  - Titel
  - Status (Abgeschlossen/Nicht abgeschlossen)
- Erinnerungsfunktion für anstehende Termine
- Übersichtliche Darstellung aller Termine und Aufgaben
- Zweiter-Monitor-Unterstützung

## Technische Details

- Programmiersprache: Go
- GUI-Framework: Fyne
- Datenbank: SQLite3
- Architektur: 
  - Hauptanwendung (GUI)
  - Daemon-Prozess für Erinnerungen

## Installation

1. Stelle sicher, dass Go installiert ist
2. Installiere die erforderlichen Abhängigkeiten:
   ```bash
   go get fyne.io/fyne/v2
   go get github.com/mattn/go-sqlite3
   ```
3. Klone das Repository
4. Kompiliere und starte die Anwendung:
   ```bash
   go run main.go
   ```

## Komponenten

- `main.go`: Hauptanwendung mit GUI
- `cmd/reminderd/main.go`: Daemon-Prozess für Erinnerungen
- `internal/reminder/`: Paket für Erinnerungsfunktionalität

## Datenbank

Die Anwendung verwendet eine SQLite-Datenbank mit zwei Tabellen:
- `appointments`: Speichert Termine
- `tasks`: Speichert Aufgaben

## Lizenz

Dieses Projekt ist unter der MIT-Lizenz lizenziert.
