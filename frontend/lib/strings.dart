/// German user-facing strings and labels. Spec: specs/README.md (UI is German), N2, N3.
library;

/// Disclaimer required on report, detail view and PDF. Spec: specs/01-legal-scope.md ("Required wording").
const disclaimerDe =
    'Automatische Prüfungen erfassen nur einen Teil möglicher Probleme und '
    'ersetzen keine Rechtsberatung. Geprüft wurde auf EU-Ebene; nationale '
    'Zusatzpflichten sind nicht enthalten.';

const ruleNamesDe = <String, String>{
  'A1': 'Textkontrast',
  'A2': 'Alternativtexte für Bilder',
  'A3': 'Beschriftung von Formularfeldern',
  'A4': 'Überschriftenstruktur',
  'A5': 'Bedienung per Tastatur',
  'A6': 'Seitensprache angegeben',
  'R1': 'Anbieterkennzeichnung (Impressum)',
  'R2': 'Datenschutzerklärung vorhanden',
  'R3': 'Cookie-Banner mit gleichwertigem Ablehnen',
  'R4': 'Keine Tracker vor Einwilligung',
  'R5': 'AGB und Widerrufsinformation vorhanden',
  'R6': 'Bestellbutton nennt Zahlungspflicht',
  'R7': 'Widerrufsfunktion',
};

String ruleNameDe(String id) => ruleNamesDe[id] ?? id;

String stateLabelDe(String state) => switch (state) {
      'passed' => 'Bestanden',
      'failed' => 'Nicht bestanden',
      'not_applicable' => 'Nicht anwendbar',
      'not_assessable' => 'Nicht automatisch beurteilbar',
      _ => state,
    };

String severityLabelDe(String severity) => switch (severity) {
      'critical' => 'Kritisch',
      'warning' => 'Warnung',
      'notice' => 'Hinweis',
      _ => severity,
    };

String formatDateDe(DateTime d) {
  final l = d.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(l.day)}.${two(l.month)}.${l.year}';
}
