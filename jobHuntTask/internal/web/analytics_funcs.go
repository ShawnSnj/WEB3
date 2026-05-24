package web

import "html/template"

func analyticsFuncMap() template.FuncMap {
	return template.FuncMap{
		"chartRefreshPath": chartRefreshPath,
		"analyticsDeltaHint": analyticsDeltaHint,
	}
}

func analyticsDeltaHint(n int, suffix string) string {
	sign := ""
	if n > 0 {
		sign = "+"
	}
	text := sign + itoa(n)
	if suffix != "" {
		return text + " " + suffix
	}
	return text
}

func chartRefreshPath(chartID string) string {
	switch chartID {
	case "chart-completion":
		return "completion"
	case "chart-carry":
		return "carry"
	case "chart-category":
		return "category"
	case "chart-productivity":
		return "productivity"
	case "chart-overdue":
		return "overdue"
	case "chart-execution":
		return "execution"
	case "chart-streak":
		return "streak"
	default:
		return "completion"
	}
}
