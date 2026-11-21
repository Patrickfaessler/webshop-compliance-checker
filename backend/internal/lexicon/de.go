package lexicon

var de = List{
	Links: map[Kind][]string{
		LegalNotice: {"impressum", "anbieterkennzeichnung", "offenlegung"},
		Privacy:     {"datenschutzerklärung", "datenschutz", "datenschutzhinweise"},
		Terms:       {"agb", "allgemeine geschäftsbedingungen", "geschäftsbedingungen"},
		Withdrawal:  {"widerrufsbelehrung", "widerrufsrecht", "widerruf", "rücktrittsrecht", "rücktritt"},
	},
	Sections: []Section{
		{"Verantwortlicher", []string{"verantwortlicher", "verantwortliche stelle", "verantwortlich"}},
		{"Zwecke und Rechtsgrundlage", []string{"rechtsgrundlage", "zweck", "art. 6"}},
		{"Empfänger", []string{"empfänger", "weitergabe", "auftragsverarbeiter"}},
		{"Speicherdauer", []string{"speicherdauer", "dauer der speicherung", "löschung", "gespeichert"}},
		{"Rechte der betroffenen Personen", []string{"ihre rechte", "auskunft", "berichtigung", "widerspruch", "datenübertragbarkeit", "betroffenenrechte"}},
		{"Beschwerderecht bei der Aufsichtsbehörde", []string{"beschwerde", "aufsichtsbehörde", "datenschutzbehörde"}},
	},
	Withdrawal:    []string{"widerrufsrecht", "widerrufen", "widerruf", "rücktrittsrecht", "rücktritt"},
	Periods:       []string{"14 tagen", "14 tage", "vierzehn tagen", "vierzehn tage", "14-tägig"},
	Exempt:        []string{"download", "digitale inhalte", "maßanfertigung", "versiegelt", "personalisiert"},
	EmailLabels:   []string{"e-mail", "email", "mail"},
	AddressLabels: []string{"anschrift", "adresse", "sitz", "postanschrift"},
	NameLabels:    []string{"inhaber", "inhaberin", "betreiber", "firmenname", "firma", "medieninhaber", "geschäftsführer", "unternehmen"},
}
