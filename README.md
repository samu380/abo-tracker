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

## Oberfläche

Links steht die **Übersicht** mit Kosten, Anzahl der Abos und der aktuellen Ansicht. In der Mitte stehen die Abos, getrennt nach aktiv und gekündigt. Rechts steht das **Detail** zum ausgewählten Abo. Bei schmalen Fenstern blendet sich erst die Übersicht und dann das Detail aus.

Die Statuszeile unten zeigt links den Modus, zum Beispiel `NORMAL`, `INSERT` oder `BEFEHL`, dazu Rückmeldungen wie „Gespeichert: Netflix“. Daneben stehen die passenden Tasten.

## Bedienung

In der Liste:

| Taste | Aktion |
| --- | --- |
| `j` `k` oder `↓` `↑` | Auswahl bewegen |
| `gg` / `G` | erstes / letztes Abo |
| `Ctrl-d` / `Ctrl-u` | halbe Seite runter / hoch |
| `n` | neues Abo |
| `e` | bearbeiten |
| `x` | kündigen oder wieder aktivieren |
| `r` | bei manueller Abbuchung: Termin um ein Intervall weiter |
| `p` | neuer Preis ab einem Monat |
| `o` | Preis- und Abbuchungsverlauf |
| `dd`, dann `y` | löschen |
| `H` | gekündigte ein- oder ausblenden |
| `[` / `]` | Übersicht / Detail ein- oder ausblenden |
| `:` oder `Ctrl-P` | Befehlspalette |
| `?` | Hilfe |
| `q` | beenden |

In der Befehlspalette filterst du die Aktionen durch Tippen, wählst mit `↑` `↓` und führst mit `Enter` aus.

Im Formular wechseln `Tab` oder `↑` `↓` das Feld. `←` `→` ändern Intervall, Kategorie und Abbuchung. `Enter` speichert aus jedem Feld, `Esc` verwirft die Eingabe.

Beträge als `12,99` oder `12.99`. Datum als `TT.MM.JJJJ`, zum Beispiel `15.11.2026`. Ein neuer Preis gilt ab einem Monat, zum Beispiel `03.2027`. Eine Kündigungsfrist von `0` Tagen blendet die Frist aus.

**Automatisch** ist die Voreinstellung. Liegt der Termin in der Vergangenheit, rückt er beim Öffnen von selbst auf den nächsten Termin, und die übersprungenen Abbuchungen landen im Verlauf. **Manuell** bleibt stehen, bis du `r` drückst. Im Verlauf steht bei jedem neuen Preis, um wie viel Prozent er vom vorherigen abweicht.

## Was die Übersicht bedeutet

**Monatlich** rechnet jedes aktive Abo auf einen Monat um. 120 € im Jahr zählen dort als 10 €. Wöchentliche Beträge werden mit 52 Wochen auf 12 Monate verteilt.

**30 Tage** ist die Summe der echten Abbuchungen, deren Termin in diesem Fenster liegt. Das Jahresabo erscheint dort nur rund um den Zahlungstag, nicht jeden Monat.

**Überfällig** sind aktive manuelle Termine, die vor heute liegen. `r` schiebt so einen Termin um genau ein Intervall weiter und merkt sich die Abbuchung.

In der Liste ist das Datum gelb, wenn die Abbuchung in den nächsten 7 Tagen kommt oder die Kündigungsfrist in den nächsten 14 Tagen endet. Rot heißt überfällig oder Frist verpasst. Den genauen Stand zeigt das Detail rechts.

## Noch nicht enthalten

Bankimport, mehrere Währungen, ein 4-Wochen-Takt und Sync zwischen Geräten.

## Später

Gelegentliche Kosten wie Inspektion, Reparatur, Wartung oder TÜV als eigener Bereich in dieser Anwendung. Nicht als zweites Programm und nicht in der Abo-Summe.
