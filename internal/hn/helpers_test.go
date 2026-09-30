package hn

import "fmt"

func fmtSscanf(s, format string, a ...any) (int, error) { return fmt.Sscanf(s, format, a...) }
