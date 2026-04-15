package ui

type Lang struct {
	code string
}

func NewLang(code string) *Lang {
	return &Lang{code: code}
}

func (l *Lang) Toggle() {
	if l.code == "EN" {
		l.code = "ET"
	} else {
		l.code = "EN"
	}
}

func (l *Lang) Code() string {
	return l.code
}

var strings = map[string]map[string]string{
	"openFolder":     {"EN": "Open Folder", "ET": "Ava kaust"},
	"openInSSMS":     {"EN": "Open in SSMS", "ET": "Ava SSMS-is"},
	"copyPath":       {"EN": "Copy Path", "ET": "Kopeeri tee"},
	"warnings":       {"EN": "Warnings", "ET": "Hoiatused"},
	"missingIndexes": {"EN": "Missing Indexes", "ET": "Puuduvad indeksid"},
	"tableScan":      {"EN": "Table Scan", "ET": "Täielik skannimine"},
	"operator":       {"EN": "Operator", "ET": "Operaator"},
	"costPct":        {"EN": "Cost %", "ET": "Kulu %"},
	"estRows":        {"EN": "Est. Rows", "ET": "Hinnangulised read"},
	"noFile":         {"EN": "Select a file from the left panel", "ET": "Vali fail vasakust paneelist"},
	"ssmsNotFound":   {"EN": "SSMS not found. Enter path:", "ET": "SSMS-i ei leitud. Sisesta tee:"},
	"deadlockVictim": {"EN": "Deadlock Victim", "ET": "Ummiku ohver"},
	"processes":      {"EN": "Processes", "ET": "Protsessid"},
	"resources":      {"EN": "Resources", "ET": "Ressursid"},
	"appTitle":       {"EN": "SQL Plan Viewer", "ET": "SQL Plaani Vaataja"},
}

func (l *Lang) T(key string) string {
	if m, ok := strings[key]; ok {
		if s, ok2 := m[l.code]; ok2 {
			return s
		}
	}
	return key
}
