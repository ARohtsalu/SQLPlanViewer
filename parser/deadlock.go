package parser

import (
	"encoding/xml"
	"os"
)

type DeadlockGraph struct {
	Victim    string
	Processes []DeadlockProcess
	Resources []DeadlockResource
	Edges     []DeadlockEdge
}

type DeadlockProcess struct {
	ID             string
	SPID           int
	Login          string
	WaitResource   string
	LogUsed        int
	IsVictim       bool
	InputBuf       string
	IsolationLevel string
	X, Y           float32
}

type DeadlockResource struct {
	ID              string
	LockType        string
	ObjectName      string
	IndexName       string
	HoBtID          string
	LockMode        string
	// Convenience: first owner and first waiter
	OwnerProcessID  string
	OwnerMode       string
	WaiterProcessID string
	WaiterMode      string
	X, Y            float32
}

type DeadlockEdge struct {
	ProcessID  string
	ResourceID string
	Mode       string
	IsOwner    bool
}

// XML structs

type xmlDeadlock struct {
	XMLName     xml.Name       `xml:"deadlock"`
	VictimList  xmlVictimList  `xml:"victim-list"`
	ProcessList []xmlDLProcess `xml:"process-list>process"`
	Resources   xmlDLResources `xml:"resource-list"`
}

type xmlVictimList struct {
	VictimProcess xmlVictimProcess `xml:"victimProcess"`
}

type xmlVictimProcess struct {
	ID string `xml:"id,attr"`
}

type xmlDLProcess struct {
	ID             string `xml:"id,attr"`
	SPID           int    `xml:"spid,attr"`
	LoginName      string `xml:"loginname,attr"`
	WaitResource   string `xml:"waitresource,attr"`
	LogUsed        int    `xml:"logused,attr"`
	IsolationLevel string `xml:"isolationlevel,attr"`
	InputBuf       string `xml:"inputbuf"`
}

type xmlDLResources struct {
	KeyLocks    []xmlDLKeyLock    `xml:"keylock"`
	PageLocks   []xmlDLPageLock   `xml:"pagelock"`
	RowLocks    []xmlDLRowLock    `xml:"ridlock"`
	ObjectLocks []xmlDLObjectLock `xml:"objectlock"`
}

type xmlDLKeyLock struct {
	ID         string       `xml:"id,attr"`
	HoBtID     string       `xml:"hobtid,attr"`
	ObjectName string       `xml:"objectname,attr"`
	IndexName  string       `xml:"indexname,attr"`
	Mode       string       `xml:"mode,attr"`
	Owners     []xmlDLOwner `xml:"owner-list>owner"`
	Waiters    []xmlDLOwner `xml:"waiter-list>waiter"`
}

type xmlDLPageLock struct {
	ID         string       `xml:"id,attr"`
	ObjectName string       `xml:"objectname,attr"`
	Mode       string       `xml:"mode,attr"`
	Owners     []xmlDLOwner `xml:"owner-list>owner"`
	Waiters    []xmlDLOwner `xml:"waiter-list>waiter"`
}

type xmlDLRowLock struct {
	ID         string       `xml:"id,attr"`
	ObjectName string       `xml:"objectname,attr"`
	Mode       string       `xml:"mode,attr"`
	Owners     []xmlDLOwner `xml:"owner-list>owner"`
	Waiters    []xmlDLOwner `xml:"waiter-list>waiter"`
}

type xmlDLObjectLock struct {
	ID         string       `xml:"id,attr"`
	ObjectName string       `xml:"objectname,attr"`
	Mode       string       `xml:"mode,attr"`
	Owners     []xmlDLOwner `xml:"owner-list>owner"`
	Waiters    []xmlDLOwner `xml:"waiter-list>waiter"`
}

type xmlDLOwner struct {
	ID   string `xml:"id,attr"`
	Mode string `xml:"mode,attr"`
}

func ParseDeadlock(path string) (*DeadlockGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root xmlDeadlock
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	dg := &DeadlockGraph{
		Victim: root.VictimList.VictimProcess.ID,
	}

	for _, p := range root.ProcessList {
		dg.Processes = append(dg.Processes, DeadlockProcess{
			ID:             p.ID,
			SPID:           p.SPID,
			Login:          p.LoginName,
			WaitResource:   p.WaitResource,
			LogUsed:        p.LogUsed,
			IsVictim:       p.ID == dg.Victim,
			InputBuf:       p.InputBuf,
			IsolationLevel: p.IsolationLevel,
		})
	}

	addLock := func(id, hobtid, objName, indexName, lockType, mode string, owners, waiters []xmlDLOwner) {
		res := DeadlockResource{
			ID:         id,
			LockType:   lockType,
			ObjectName: objName,
			IndexName:  indexName,
			HoBtID:     hobtid,
			LockMode:   mode,
		}
		if len(owners) > 0 {
			res.OwnerProcessID = owners[0].ID
			res.OwnerMode = owners[0].Mode
		}
		if len(waiters) > 0 {
			res.WaiterProcessID = waiters[0].ID
			res.WaiterMode = waiters[0].Mode
		}
		dg.Resources = append(dg.Resources, res)

		for _, o := range owners {
			dg.Edges = append(dg.Edges, DeadlockEdge{ProcessID: o.ID, ResourceID: id, Mode: o.Mode, IsOwner: true})
		}
		for _, w := range waiters {
			dg.Edges = append(dg.Edges, DeadlockEdge{ProcessID: w.ID, ResourceID: id, Mode: w.Mode, IsOwner: false})
		}
	}

	for _, l := range root.Resources.KeyLocks {
		addLock(l.ID, l.HoBtID, l.ObjectName, l.IndexName, "Key Lock", l.Mode, l.Owners, l.Waiters)
	}
	for _, l := range root.Resources.PageLocks {
		addLock(l.ID, "", l.ObjectName, "", "Page Lock", l.Mode, l.Owners, l.Waiters)
	}
	for _, l := range root.Resources.RowLocks {
		addLock(l.ID, "", l.ObjectName, "", "Row Lock", l.Mode, l.Owners, l.Waiters)
	}
	for _, l := range root.Resources.ObjectLocks {
		addLock(l.ID, "", l.ObjectName, "", "Object Lock", l.Mode, l.Owners, l.Waiters)
	}

	return dg, nil
}

// FindProcessBySPID returns the SPID for a given process ID string.
func FindProcessSPID(dg *DeadlockGraph, procID string) int {
	for _, p := range dg.Processes {
		if p.ID == procID {
			return p.SPID
		}
	}
	return 0
}
