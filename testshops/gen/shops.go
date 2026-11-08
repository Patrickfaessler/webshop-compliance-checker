package main

// The three fictional test shops (specs/09-testing.md, T-3). All company data is invented.
// Every planted defect is a flag here, so this file is the single list of what each shop does wrong.

type Product struct {
	Slug, Name, Desc, Price, Color string
}

type Company struct {
	Name, Owner, Street, City, Email, Phone, VAT, Register string
}

// Defects switches on planted problems. Field comments name the rule that should find them.
type Defects struct {
	NoLangOn              map[string]bool // A6: pages whose <html> has no lang attribute
	LowContrastHints      bool            // A1: price hints in #aaaaaa on white
	LowContrastOn         map[string]bool // A1: a grey note (#9a9a9a) only on these pages
	ProductImgNoAlt       bool            // A2: category pages show product images without alt
	FilenameAltProduct    string          // A2: this product's image has alt = file name
	SearchNoLabel         bool            // A3: header search field without any label
	NewsletterPlaceholder bool            // A3: footer newsletter field with placeholder only
	SkipLevelOn           map[string]bool // A4: h1 followed directly by h3 on these pages
	DoubleH1OnProducts    bool            // A4: product pages have two h1
	EmptyHeadingOnCart    bool            // A4: empty h2 on the cart page
	SkipH2ToH4OnCategory  bool            // A4: category pages jump from h2 to h4

	// Legal defects, planted now for the R-rules that are not implemented yet (documented in expected.json).
	ImpressumNoEmail   bool   // R1
	ImpressumNoVAT     bool   // R1 (conditional field)
	PrivacyShort       bool   // R2
	CookieNoReject     bool   // R3: banner offers only "Akzeptieren" and "Einstellungen"
	OrderButton        string // R6: label of the final checkout button
	NoWithdrawalPage   bool   // R5: no withdrawal information, no link
	WithdrawalFunction bool   // R7: a clearly labelled "Vertrag widerrufen" function exists
}

type Shop struct {
	Slug, Name, Tagline, Kind string
	Primary, Accent           string // colours; all chosen with >= 4.5:1 contrast on white
	Company                   Company
	Products                  []Product
	Categories                [2]string
	Defects                   Defects
	Expected                  map[string]string // rule id -> expected state for the implemented rules
	ExpectedScore             int
	Planted                   []Planted
}

type Planted struct {
	Page string `json:"page"`
	Rule string `json:"rule"`
	What string `json:"what"`
}

func shops() []Shop {
	return []Shop{
		{
			Slug: "teehaus", Name: "Teehaus Bergkristall", Tagline: "Lose Tees aus kleinen Gärten", Kind: "exemplary",
			Primary: "#1f5130", Accent: "#8a4b08",
			Company: Company{Name: "Teehaus Bergkristall e.U. (fiktiv)", Owner: "Erika Musterfrau", Street: "Musterweg 1",
				City: "5020 Salzburg, Österreich", Email: "hallo@teehaus-bergkristall.example", Phone: "+43 662 000000",
				VAT: "ATU00000000 (fiktiv)", Register: "FN 000000a, Landesgericht Salzburg (fiktiv)"},
			Categories: [2]string{"Grüner Tee", "Kräutertee"},
			Products: []Product{
				{"sencha", "Sencha Bergnebel", "Frischer grüner Tee mit milder Süße.", "8,90 €", "#4f8a5b"},
				{"minze", "Alpenminze", "Kräftige Pfefferminze aus dem Lungau.", "6,50 €", "#3f7f74"},
				{"rooibos", "Rooibos Vanille", "Koffeinfrei, mit echter Vanille.", "7,20 €", "#9c5a2e"},
			},
			Defects:       Defects{OrderButton: "Zahlungspflichtig bestellen", WithdrawalFunction: true},
			Expected:      map[string]string{"A1": "passed", "A2": "passed", "A3": "passed", "A4": "passed", "A6": "passed"},
			ExpectedScore: 100,
		},
		{
			Slug: "radwerk", Name: "Radwerk Alpin", Tagline: "Fahrradzubehör für Berg und Stadt", Kind: "many defects",
			Primary: "#0b4a7a", Accent: "#b3261e",
			Company: Company{Name: "Radwerk Alpin GmbH (fiktiv)", Owner: "Max Mustermann", Street: "Beispielgasse 7",
				City: "6020 Innsbruck, Österreich", Email: "service@radwerk-alpin.example", Phone: "+43 512 000000",
				VAT: "ATU11111111 (fiktiv)", Register: "FN 111111b, Landesgericht Innsbruck (fiktiv)"},
			Categories: [2]string{"Beleuchtung", "Taschen"},
			Products: []Product{
				{"lampe", "Frontlicht Gipfel 800", "800 Lumen, USB-C, wasserdicht.", "49,00 €", "#2b5d8a"},
				{"tasche", "Gepäckträgertasche Talfahrt", "22 Liter, Rolltop, reflektierend.", "69,00 €", "#6b3fa0"},
				{"schloss", "Faltschloss Felsfest", "Stahl, 90 cm, mit Halterung.", "39,90 €", "#5a5a5a"},
			},
			Defects: Defects{
				NoLangOn:              map[string]bool{"index": true, "kategorie-1": true, "kategorie-2": true, "produkt-lampe": true, "produkt-tasche": true, "produkt-schloss": true},
				LowContrastHints:      true,
				ProductImgNoAlt:       true,
				FilenameAltProduct:    "tasche",
				SearchNoLabel:         true,
				NewsletterPlaceholder: true,
				SkipLevelOn:           map[string]bool{"index": true},
				DoubleH1OnProducts:    true,
				EmptyHeadingOnCart:    true,
				ImpressumNoEmail:      true,
				ImpressumNoVAT:        true,
				PrivacyShort:          true,
				CookieNoReject:        true,
				OrderButton:           "Bestellen",
				NoWithdrawalPage:      true,
			},
			Expected:      map[string]string{"A1": "failed", "A2": "failed", "A3": "failed", "A4": "failed", "A6": "failed"},
			ExpectedScore: 0,
		},
		{
			Slug: "seifen", Name: "Seifenmanufaktur Lavendel", Tagline: "Handgemachte Seifen aus dem Salzkammergut", Kind: "mixed",
			Primary: "#5b2a86", Accent: "#7a3b00",
			Company: Company{Name: "Seifenmanufaktur Lavendel OG (fiktiv)", Owner: "Sophie Beispiel", Street: "Am Teststeg 3",
				City: "4820 Bad Ischl, Österreich", Email: "post@seifen-lavendel.example", Phone: "+43 6132 000000",
				VAT: "ATU22222222 (fiktiv)", Register: "FN 222222c, Landesgericht Wels (fiktiv)"},
			Categories: [2]string{"Körperseifen", "Haarseifen"},
			Products: []Product{
				{"lavendel", "Lavendelseife", "Mit Öl aus eigenem Anbau.", "5,90 €", "#7d5ba6"},
				{"honig", "Honig-Hafer-Seife", "Mild, für empfindliche Haut.", "6,20 €", "#b07a1f"},
				{"zirbe", "Zirbenseife", "Duftet nach Bergwald.", "6,50 €", "#4a6b3a"},
			},
			Defects: Defects{
				LowContrastOn:        map[string]bool{"agb": true},
				FilenameAltProduct:   "lavendel",
				SkipH2ToH4OnCategory: true,
				OrderButton:          "Jetzt kaufen",
			},
			Expected:      map[string]string{"A1": "failed", "A2": "failed", "A3": "passed", "A4": "failed", "A6": "passed"},
			ExpectedScore: 40,
		},
	}
}
