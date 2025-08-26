package notifier

// Static mapping for human-friendly names.
// Extend here if you add more companies/services/forms.
var (
	companyNames = map[string]string{
		"780413": "Неваляшка",
	}
	serviceNames = map[string]string{
		"15728488": "Город с инструктором",
	}
	formNames = map[string]string{
		"n841217": "Город с инструктором",
	}
)

func CompanyName(id string) (string, bool) {
	name, ok := companyNames[id]
	return name, ok
}

func ServiceName(id string) (string, bool) {
	name, ok := serviceNames[id]
	return name, ok
}

func FormName(id string) (string, bool) {
	name, ok := formNames[id]
	return name, ok
}
