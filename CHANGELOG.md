# SQLPlanViewer — Changelog

---

## Build 2026-04-17 — Värvid, resize-refit, Performance Studio browse

### Visuaalne
- **Kolmeastmeline kuluvärvistus**: noodivärvid on nüüd kolme taseme järgi
  - 0–10%: soe kollakaskuldne taust
  - 10–25%: amberkollane taust + amberpiirjoon
  - ≥25%: matt punakasroheline taust + tume punane piirjoon (ei kriiska)
- Varem oli ainult binaarne (tavaline / punane)

### Akna suuruse käsitlemine
- **Automaatne refit ekraanide vahel**: kui akna suurus muutub rohkem kui 25% (nt liigutad teisele monitorile), skaleeritakse plaan automaatselt uuele suurusele sobivaks
- `fittedSize` jälgib, millise suuruse juures viimati fit tehti

### Performance Studio
- **Browse nupp dialoogis**: esimesel avamisel küsitava tee dialoogile lisati "Browse..." nupp, mis avab `.exe` failibrauseri — ei pea teed käsitsi trükkima

---

## Build 2026-04-16 — Jõudlus + UX parandused

### Kiirus
- **Parser single-pass**: `parseRelOpExtras` + `findDirectRelOps` asendatud uhega (`parseRelOpInnerXML`) — iga RelOp InnerXML parsitakse korra, mitte kahel korral. Eemaldati ka redundantne namespace-strippimne InnerXML-ist (juba eeaedatu ylatasandil).
- **nodeHeight caching**: `nodeHeight()` tulemus salvestatakse `RelOp.CachedHeight` valjale layouti ajal. Varemalt arvutati igal drawEdges/drawNodes/nodeAtPos kutsel uuesti (tuhandeid kordi hiirega liikudes).
- **Scroll container eemaldatud**: PlanCanvas ei ole enam `container.NewScroll` sees — haldab ise zoom (scroll) ja pan (drag). Elimineerib MinSize tagasiside-tsyklid ja liigse layout-kihi.
- **Renderer: slice reuse**: `r.objects = r.objects[:0]` asemele `r.objects = nil` — taaskasutab olemasolevat backing array-t, vahendab GC survet.
- **filepath.WalkDir**: kaustaskaneerimine kasutab `WalkDir` (ei kutsu `os.Lstat` igale failile) asemele `Walk`.

### Tooltip
- **widget.PopUp asendatud canvas-objektidega**: tooltip joonistatakse otse renderi sees kui `canvas.Rectangle` + `canvas.Text` objektid. Need ei puua hiiresundmusi kinni, seega hover/click toimib alati korrektselt.
- **Hover loogika**: tooltip ilmub sõlmele minnes, kaob ära minnes. MouseOut ei peata tooltippi (vältib popup/hover tsüklit).
- **Click-pin**: klikk sõlmel kinnitab tooltippi; klikk uuesti voi tühjale alale sulgeb. Liikumine uuele sõlmele sulgeb vana.
- **Tume taust**: tooltip kasutab tumedat tausta (`#1E222B`) — tekst on loetav dark teemas.

### Failide avamine
- **Faili valimine dialoogist**: "Open" nupp kasutab nüüd `dialog.NewFileOpen` filtriga `.sqlplan`/`.xdl`. Faili valimisel laetakse selle kataloog ja fail valitakse automaatselt.
- **LoadFolderAndSelect**: uus FileTree meetod — laeb kausta ja valib konkreetse faili.

### Tabide loogika
- **"All Queries" tab eemaldatud**: see lõi N mini-graafikut korraga ja oli peamine aegluse põhjus.
- **Lazy tab loading**: iga tabi sisu luuakse alles esimesel klikil, mitte kõik korraga.
- **Kalleim päring automaatselt aktiivne**: avatakse `MostExpensiveIndex` tab, mitte alati Q1.
- **Päringu tekst loetav**: `widget.NewMultiLineEntry + Disable()` asendatud `widget.NewLabel`-iga (teema-korrektne värv dark teemas).

### Auto-fit
- **FitToWindow ainult esimesel korral**: akna suuruse muutmine ei skalleeri plaani ümber. Zoom/pan jääb stabiilseks.
- **Reset nupp** taastab fit-to-window.

---

## [Unreleased / Current build] — 2026-04-15

### Update 14 — Erik Darlingi PlanViewer port
- **Layout suund muudetud**: TOP-DOWN → VASAK-PAREM (root vasakul, lapsed paremal) — identne SSMS-iga
- **Vanema Y positsioon**: enam ei tsentreeritud laste vahel — vanem joondub **esimese lapsega** (SSMS-stiil, ported from `PlanLayoutEngine.cs`)
- **Muutuv noodi kõrgus**: fikseeritud 65px → arvutuslik (ikoon 36 + nimi 17 + cost 17 + padding 12 = min 90px, +15px objekti rea kohta)
- **L-kujulised nooled (elbow connector)**: sirge joon → 3-segmendiline L-kujuline ühendus (parent parem-center → midX → child vasak-center), paksus logaritmiline (`max(2, min(floor(log(rows)), 12))`)
- **PNG ikoonid**: Unicode emoji-d → 113 SSMS-stiilis PNG ikooni (32×32), kopeeritud Eriku lähtekoodist, sisseehitatud binaarisse `//go:embed`
- **Tume teema**: `app.Settings().SetTheme(theme.DarkTheme())`; noodivärvid `DarkTheme.axaml` järgi:
  - Tavaline nood: bg `#22252D`, border `#3A3D45`, tekst `#E4E6EB`
  - Kallis nood (≥25%): punakaspõhi + OrangeRed `#FF4500` border
  - Cost tekst: oranž ≥25%, oranžpunane ≥50%
  - Noolte värv: `#6B7280`
- Uued failid: `ui/iconmap.go`, `ui/icons_embed.go`, `ui/planicons/` (113 PNG)
- **Veaparandus**: `app.New()` → `app.NewWithID("io.github.arohtsalu.sqlplanviewer")` (Fyne Preferences API nõue)

---

## Build 2026-04-15 (12:24) — Updates 7 + 8 + 9

### Update 9 — Seaded, klaviatuur, staatusriba
- `settings.json` (SSMSPath, PerformanceStudioPath, LastFolder, Language) — laetakse käivitusel, salvestatakse sulgemisel
- Viimane kaust taastatakse automaatselt käivitusel
- Klaviatuurinavigatsioon: ↑↓ nooleklahvid liiguvad faililoetelus (`Canvas.SetOnTypedKey`)
- Akna tiitliriba uueneb aktiivse faili nimega
- Staatusriba: `failinimi | Fail N / Kokku | ↑↓ nooleklahvid`
- Toolbar: "Ava SSMS-is" nupp (otsib SSMS-i automaatselt, küsib teed kui ei leia, salvestab settings.json-i)
- Toolbar: "Kopeeri tee" nupp
- `FileTree`: `SelectNext()`, `SelectPrevious()`, `SelectedFile()`, `Count()`, `CurrentFolder()`
- Uued failid: `ui/settings.go`, `ui/statusbar.go`

### Update 8 — Hover tooltip
- `PlanCanvas` implementeerib `desktop.Hoverable` (`MouseIn`, `MouseOut`, `MouseMoved`)
- Hiirega noodi peale → SSMS-stiilis kollane popup infoga (IO cost, CPU cost, tabel/indeks, hoiatused)
- `nodeAtPos()` hit-test skaalaga koordinaatides
- `showTooltip()` / `hideTooltip()` — `widget.PopUp` haldus
- `allNodes []*parser.RelOp` — flat nimekiri kõigist noodidest tooltip otsinguks
- Uus fail: `ui/tooltip.go` (`BuildTooltipContent()`, `operatorDescription()`)

### Update 7 — Layout + auto-fit + vaikimisi tab
- `FitToWindow(availW, availH float32)` — arvutab automaatselt skaala et graaf mahub aknasse
- `Resize()` override — kutsub FitToWindow akna suuruse muutumisel
- Reset nupp kutsub FitToWindow (mitte hardcoded Scale=1.0)
- "All Queries" on vaikimisi aktiivne tab (`SelectIndex(0)`)
- Mini-graafikud "All Queries" vaates: 250px kõrgus, auto-fit, mitte-interaktiivne

---

## Varasemad builds (Updates 1–6)

### Update 6 — Parser fix (kriitilised vead)
- `findDirectRelOps`: eemaldati depth check — `DecodeElement` tarbib alampu automaatselt, nii leiab kõik RelOp sõlmed (1 sõlm → 17 sõlme testfailil)
- XDL deadlock parser: ohver loetakse `<victim-list><victimProcess id="..."/>` alt, mitte `<deadlock>` atribuudist

### Update 5 — Visuaalsed parandused
- Graafi sõlme kujunduse täiendused (värvid, tekst, border)
- Tooltip põhistruktuur

### Update 2 — Deadlock graaf
- `parser/deadlock.go` — XDL formaadi parser
- `ui/deadlockgraph.go` — visuaalne deadlock graaf (lohistatavad sõlmed, `desktop.Mouseable`)
- `BuildDeadlockInfoPanel()` — struktureeritud info paneel (SPID, login, SQL, lukud)

### Update 1 — Algseis
- Projekti struktuur loodud (Go module, Fyne v2)
- `parser/sqlplan.go` — ShowPlanXML parser (namespace stripping, RelOp rekursiivne lugemine)
- `ui/plangraph.go` — esimene graafi canvas
- `ui/planview.go`, `ui/filetree.go`, `ui/toolbar.go`, `ui/lang.go`
- `main.go` — HSplit layoutiga peaken
- Git init, push GitHub ARohtsalu/SQLPlanViewer `master`
