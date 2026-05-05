package components

func inputErrorClass(hasError bool) string {
	if hasError {
		return "input-error"
	}
	return ""
}

func inputBorderClass(hasError bool) string {
	if hasError {
		return "border-rose-400"
	}
	return "border-slate-200"
}
