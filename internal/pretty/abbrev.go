package pretty

import "fmt"

func Abbrev(s string, ranges ...int) Abbreviated {
	MaxLen := 12
	CutTo := 12
	if len(ranges) >= 2 {
		MaxLen, CutTo = ranges[0], ranges[1]
	} else if len(ranges) == 1 {
		MaxLen, CutTo = ranges[0], ranges[0]
	}
	return Abbreviated{
		Original: s,
		MaxLen:   MaxLen,
		CutTo:    CutTo,
	}
}

type Abbreviated struct {
	Original string
	MaxLen   int
	CutTo    int
}

func (s Abbreviated) String() string {
	if len(s.Original) > s.MaxLen {
		return fmt.Sprintf("%s…", s.Original[:s.CutTo])
	}
	return s.Original
}
