package parser

import (
	"encoding/xml"
	"os"
)

type DeadlockGraph struct {
	Victim    string
	Processes []DeadlockProcess
	Resources []DeadlockResource
}

type DeadlockProcess struct {
	ID           string
	SPID         int
	Login        string
	WaitResource string
	IsVictim     bool
	QueryText    string
}

type DeadlockResource struct {
	Type       string
	ObjectName string
	Mode       string
}

// --- XML structs ---

type xmlDeadlock struct {
	XMLName     xml.Name          `xml:"deadlock"`
	Victim      string            `xml:"victim,attr"`
	ProcessList []xmlDLProcess    `xml:"process-list>process"`
	ResourceList xmlDLResourceList `xml:"resource-list"`
}

type xmlDLProcess struct {
	ID           string `xml:"id,attr"`
	SPID         int    `xml:"spid,attr"`
	LoginName    string `xml:"loginname,attr"`
	WaitResource string `xml:"waitresource,attr"`
	InputBuf     string `xml:"inputbuf"`
}

type xmlDLResourceList struct {
	PageLocks   []xmlDLLock `xml:"pagelock"`
	RowLocks    []xmlDLLock `xml:"ridlock"`
	ObjectLocks []xmlDLLock `xml:"objectlock"`
	KeyLocks    []xmlDLLock `xml:"keylock"`
}

type xmlDLLock struct {
	ObjectName string `xml:"objectname,attr"`
	Mode       string `xml:"mode,attr"`
	Type       string `xml:"type,attr"`
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
		Victim: root.Victim,
	}

	for _, p := range root.ProcessList {
		dg.Processes = append(dg.Processes, DeadlockProcess{
			ID:           p.ID,
			SPID:         p.SPID,
			Login:        p.LoginName,
			WaitResource: p.WaitResource,
			IsVictim:     p.ID == root.Victim,
			QueryText:    p.InputBuf,
		})
	}

	addLocks := func(locks []xmlDLLock, lockType string) {
		for _, l := range locks {
			t := lockType
			if l.Type != "" {
				t = l.Type
			}
			dg.Resources = append(dg.Resources, DeadlockResource{
				Type:       t,
				ObjectName: l.ObjectName,
				Mode:       l.Mode,
			})
		}
	}

	addLocks(root.ResourceList.PageLocks, "Page Lock")
	addLocks(root.ResourceList.RowLocks, "Row Lock")
	addLocks(root.ResourceList.ObjectLocks, "Object Lock")
	addLocks(root.ResourceList.KeyLocks, "Key Lock")

	return dg, nil
}
