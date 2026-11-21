package lexicon

var en = List{
	Lang: "en",
	Links: map[Kind][]string{
		LegalNotice: {"legal notice", "imprint", "legal-notice", "site notice", "company information"},
		Privacy:     {"privacy policy", "privacy notice", "privacy", "privacy-policy", "data protection"},
		Terms:       {"terms and conditions", "terms & conditions", "terms of sale", "terms of service", "terms-of-service", "terms"},
		Withdrawal:  {"right of withdrawal", "withdrawal", "cancellation policy", "cancellation", "returns policy"},
	},
	Sections: []Section{
		{"Verantwortlicher", []string{"data controller", "controller", "responsible for"}},
		{"Zwecke und Rechtsgrundlage", []string{"legal basis", "purpose", "art. 6", "lawful basis"}},
		{"Empfänger", []string{"recipients", "third parties", "processors", "share your"}},
		{"Speicherdauer", []string{"retention", "how long", "retain", "stored for"}},
		{"Rechte der betroffenen Personen", []string{"your rights", "right of access", "erasure", "rectification", "data portability"}},
		{"Beschwerderecht bei der Aufsichtsbehörde", []string{"supervisory authority", "lodge a complaint", "complaint"}},
	},
	Withdrawal:    []string{"right of withdrawal", "withdraw from", "withdrawal", "cancel the contract", "cancel this contract", "right to cancel"},
	Periods:       []string{"14 days", "fourteen days", "14-day", "fourteen-day"},
	Exempt:        []string{"download", "digital content", "made to order", "personalised", "personalized", "sealed"},
	EmailLabels:   []string{"e-mail", "email", "mail"},
	AddressLabels: []string{"address", "registered office", "head office"},
	NameLabels:    []string{"owner", "operator", "company name", "operated by", "director"},
}
