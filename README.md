# Abo-Tracker

Kleine Terminal-Oberfläche für Abos und andere wiederkehrende Kosten. Alles liegt in einer JSON-Datei auf dem Rechner.

## Start

```bash
go run ./cmd/abo
```

Zum Ausprobieren mit einer eigenen Datei im Projektordner:

```bash
go run ./cmd/abo --data ./subscriptions.json
```

Ohne `--data` liegt die Datei unter macOS in `~/Library/Application Support/abo-tracker/subscriptions.json`.

Programm bauen:

```bash
go build -o abo ./cmd/abo
./abo
```

## Bedienung

In der Liste:

| Taste | Aktion |
| --- | --- |
| `↑` `↓` | Auswahl bewegen, auch `k` und `j` |
| `a` | neues Abo |
| `e` | bearbeiten |
| `c` | kündigen oder wieder aktivieren |
| `n` | bei manueller Abbuchung: Termin um ein Intervall weiter |
| `p` | neuer Preis ab einem Monat |
| `h` | Preis- und Abbuchungsverlauf |
| `d` dann `j` | löschen |
| `f` | nur aktive Abos, nochmal `f` für alle |
| `?` | Hilfe |
| `q` | beenden |

Im Formular: `↑` `↓` wechseln das Feld, `←` `→` ändern Intervall, Kategorie und Abbuchung. `enter` geht zum nächsten Feld. Auf **Speichern** sichert `enter`. `esc` verwirft die Eingabe.

Beträge als `12,99` oder `12.99`. Datum als `TT.MM.JJJJ`, zum Beispiel `15.11.2026`. Ein neuer Preis gilt ab einem Monat, zum Beispiel `03.2027`. Eine Kündigungsfrist von `0` Tagen blendet die Frist aus.

**Automatisch** ist die Voreinstellung. Liegt der Termin in der Vergangenheit, rückt er beim Öffnen von selbst auf den nächsten Termin, und die übersprungenen Abbuchungen landen im Verlauf. **Manuell** bleibt stehen, bis du `n` drückst. Im Verlauf steht bei jedem neuen Preis, um wie viel Prozent er vom vorherigen abweicht.

## Was die Übersicht bedeutet

**Monatlich aktiv** rechnet jedes aktive Abo auf einen Monat um. 120 € im Jahr zählen dort als 10 €. Wöchentliche Beträge werden mit 52 Wochen auf 12 Monate verteilt.

**Nächste 30 Tage** ist die Summe der echten Abbuchungen, deren Termin in diesem Fenster liegt. Das Jahresabo erscheint dort nur rund um den Zahlungstag, nicht jeden Monat.

**Überfällig** sind aktive manuelle Termine, die vor heute liegen. `n` schiebt so einen Termin um genau ein Intervall weiter und merkt sich die Abbuchung.

Liegt die Kündigungsfrist in den nächsten 14 Tagen oder ist sie schon vorbei, steht das in der Zeile unter dem Abo.

## Noch nicht enthalten

Bankimport, mehrere Währungen, ein 4-Wochen-Takt und Sync zwischen Geräten.

## Später

Gelegentliche Kosten wie Inspektion, Reparatur, Wartung oder TÜV als eigener Bereich in dieser Anwendung. Nicht als zweites Programm und nicht in der Abo-Summe.
