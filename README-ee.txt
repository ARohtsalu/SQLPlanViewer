# SQLPlanViewer — Eestikeelne juhend

Kerge ja kiire SQL Serveri täitmisplaanide vaataja Windowsile — üks exe, ei vaja installimist.

Mõeldud suurte plaanifailide kogude kiireks läbivaatamiseks, sarnaselt sellele kuidas IrfanView käib piltidest läbi.

---

## Funktsioonid

### Failide haldus
- Avab kausta (.sqlplan ja .xdl failid)
- Nooleklahvid ↑↓ liiguvad failide vahel
- Viimane kaust taastatakse käivitusel automaatselt
- Kopeeri faili tee lõikelauale

### Täitmisplaanid (.sqlplan)
- Kuvab kõik päringud tabidena — kalleim päring avatakse automaatselt
- Tabid värvitud kulu järgi: hall (0%), soe kollane (0–10%), amber (10–25%), matt punane (≥25%)
- Visuaalne puu: SSMS-stiilis vasak-parem paigutus, L-kujulised ühendused, logaritmilise paksusega nooled
- 113 SSMS-stiilis PNG ikooni operaatoritüüpide jaoks
- Hover tooltip: kulu %, I/O ja CPU, objekt, predikaat, veergude loend, hoiatused
- Klikk sõlmel kinnitab tooltippi; uuesti klikk või tühjale alale klikk sulgeb
- Zoom: kerimine, +/- nupud, automaatne fit aknasse
- Akna suuruse muutumisel (nt teisele monitorile liikumisel) refit automaatselt

### Deadlock graafikud (.xdl)
- Visuaalne deadlock graaf lohistatavate sõlmedega
- Protsessid: SPID, login, isolatsioonitase, SQL tekst
- Ressursid: ootavad vs. omavad lukud

### Hoiatused
- Puuduvad indeksid (iga tabel eraldi, impact%)
- Tabeliskannid
- Statistika puudub, TempDB mälukadu, ühenduspredikaat puudub

### Tööriistariba
- Ava SSMS-is: leiab SSMS automaatselt, küsib tee kui ei leia
- Ava Performance Studios (Erik Darlingi tööriist): Browse-nupp tee valimiseks
- Keele lüliti: ET | EN

---

## Nõuded

- Windows 10/11 (64-bit)
- SQL Server Management Studio (valikuline, "Open in SSMS" jaoks)
- Performance Studio — https://github.com/erikdarlingdata/PerformanceStudio (valikuline)

---

## Ehitamine

Vajalik: Go 1.22+, GCC (MSYS2 ucrt64 või mingw64), Fyne v2.

  MSYS2 ucrt64 shellis:
    pacman -S mingw-w64-ucrt-x86_64-gcc

  Projekti juurkaustas:
    go build -ldflags="-H windowsgui" -o sqlplanviewer.exe .

Tulemus on üksainus sqlplanviewer.exe ilma väliste sõltuvusteta.

---

## Seaded

settings.json (salvestatakse %APPDATA%\sqlplanviewer\):

  ssmsPath               — SSMS käivitusfaili tee
  performanceStudioPath  — PlanViewer.App.exe tee
  lastFolder             — Viimati avatud kaust
  language               — EN või ET

---

## Projekt

Kirjutatud Go + Fyne v2 (https://fyne.io/).
Inspireeritud Erik Darlingi Performance Studio visuaalist ja SSMS-i plaanivaatajast.
