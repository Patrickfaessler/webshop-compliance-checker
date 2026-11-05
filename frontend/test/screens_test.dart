import 'package:compliance_checker/api/api_client.dart';
import 'package:compliance_checker/main.dart';
import 'package:compliance_checker/screens/start_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_api.dart';

Future<void> openApp(WidgetTester tester, FakeApi api) async {
  await tester.pumpWidget(App(api: api));
  await tester.pumpAndSettle();
}

void main() {
  group('StartPage', () {
    // Spec: F1
    test('normalizeUrl adds https for bare hosts', () {
      expect(normalizeShopUrl('www.shop.at'), 'https://www.shop.at');
      expect(normalizeShopUrl('  http://shop.at '), 'http://shop.at');
      expect(normalizeShopUrl('   '), '');
    });

    // Spec: F1 — empty input is refused with a German message, no request is made.
    testWidgets('empty URL shows an error and does not call the API',
        (tester) async {
      final api = FakeApi();
      await openApp(tester, api);
      await tester.tap(find.text('Jetzt prüfen'));
      await tester.pump();
      expect(
          find.text('Bitte die Adresse Ihres Shops eingeben.'), findsOneWidget);
      expect(api.createdUrl, isNull);
    });

    // Spec: F1 — at least one area is required.
    testWidgets('no area selected shows an error', (tester) async {
      await openApp(tester, FakeApi());
      await tester.enterText(find.byType(TextField), 'shop.at');
      await tester.tap(find.text('Barrierefreiheit (WCAG 2.1 AA)'));
      await tester.tap(find.text('Rechtliche Pflichtangaben (EU-Ebene)'));
      await tester.tap(find.text('Jetzt prüfen'));
      await tester.pump();
      expect(find.text('Bitte mindestens einen Prüfbereich auswählen.'),
          findsOneWidget);
    });

    // Spec: F1, F11 — submit creates the scan with normalized URL and opens /r/{id}.
    testWidgets('submit creates a scan and navigates to the result page',
        (tester) async {
      final api = FakeApi(statuses: [statusOf('new123', 'queued')]);
      await openApp(tester, api);
      await tester.enterText(find.byType(TextField), 'mein-shop.at');
      await tester.tap(find.text('Jetzt prüfen'));
      await tester.pump();
      await tester.pump();
      expect(api.createdUrl, 'https://mein-shop.at');
      expect(api.createdAreas, ['accessibility', 'legal']);
      expect(find.text('Prüfergebnis'), findsOneWidget);
      expect(
          find.text('Ihre Prüfung wartet in der Schlange …'), findsOneWidget);
      // leave the page so the polling timer is cancelled
      await tester.pumpWidget(const SizedBox());
    });

    // Spec: F1 / C-SSRF — a server problem title is shown to the user.
    testWidgets('API problem message is displayed', (tester) async {
      final api = FakeApi(
          createError: const ApiException(
              'Diese Adresse konnte nicht geprüft werden.',
              status: 400));
      await openApp(tester, api);
      await tester.enterText(find.byType(TextField), 'http://127.0.0.1');
      await tester.tap(find.text('Jetzt prüfen'));
      await tester.pump();
      await tester.pump();
      expect(find.text('Diese Adresse konnte nicht geprüft werden.'),
          findsOneWidget);
    });
  });

  group('ResultPage', () {
    Future<void> openResult(WidgetTester tester, FakeApi api) async {
      await tester.pumpWidget(App(api: api));
      await tester.pumpAndSettle();
      // jump straight to a result by creating a scan
      await tester.enterText(find.byType(TextField), 'shop.at');
      await tester.tap(find.text('Jetzt prüfen'));
      await tester.pump();
      await tester.pump();
    }

    // Spec: F7, S-2, S-3, N3 — score, rule results, findings, disclaimer and wording.
    testWidgets('finished scan shows score, rules, findings, disclaimer',
        (tester) async {
      // A ListView only builds visible items, so use a tall viewport.
      tester.view.physicalSize = const Size(900, 3000);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      final api = FakeApi(
        statuses: [
          statusOf('new123', 'done',
              score: 60, expiresAt: DateTime.utc(2026, 12, 5, 12))
        ],
        findings: const ScanFindings(
          ruleResults: [
            RuleResult(
                ruleId: 'A6',
                state: 'failed',
                decidedBy: 'structural',
                pagesAffected: 2),
            RuleResult(
                ruleId: 'A1',
                state: 'passed',
                decidedBy: 'structural',
                pagesAffected: 0),
            RuleResult(
                ruleId: 'R3',
                state: 'not_assessable',
                decidedBy: '',
                pagesAffected: 0),
          ],
          findings: [
            Finding(
                id: 'f1',
                ruleId: 'A6',
                severity: 'notice',
                url: 'https://shop.at/',
                summaryDe: 'Sprache fehlt.'),
          ],
        ),
      );
      await openResult(tester, api);
      await tester.pumpAndSettle();

      expect(find.text('Anteil bestandener automatischer Prüfungen'),
          findsOneWidget);
      expect(find.text('60 %'), findsOneWidget);
      expect(find.textContaining('keine Aussage über vollständige Konformität'),
          findsOneWidget);
      expect(find.text('A6 · Seitensprache angegeben'), findsOneWidget);
      expect(
          find.text('Nicht bestanden · 2 Seite(n) betroffen'), findsOneWidget);
      expect(find.text('Nicht automatisch beurteilbar'), findsOneWidget);
      expect(find.text('Gefundene Probleme (1)'), findsOneWidget);
      expect(find.text('Hinweis · A6 Seitensprache angegeben'), findsOneWidget);
      expect(find.text('Sprache fehlt.'), findsOneWidget);
      expect(find.text('DISCLAIMER-VOM-SERVER'), findsOneWidget);
      expect(find.textContaining('bis 05.12.2026 gültig'), findsOneWidget);
    });

    // Spec: S-1 — no score is shown as "nicht berechenbar", never as compliant.
    testWidgets('missing score reads "nicht berechenbar"', (tester) async {
      await openResult(tester, FakeApi(statuses: [statusOf('new123', 'done')]));
      await tester.pumpAndSettle();
      expect(find.text('nicht berechenbar'), findsOneWidget);
      expect(find.textContaining('%'), findsNothing);
    });

    // Spec: F7 — polling moves from running to done and stops.
    testWidgets('polls until the scan is done', (tester) async {
      final api = FakeApi(statuses: [
        statusOf('new123', 'running', done: 1, total: 4),
        statusOf('new123', 'done', score: 100),
      ]);
      await openResult(tester, api);
      expect(find.text('Ihr Shop wird geprüft …'), findsOneWidget);
      expect(find.text('1 von 4 Seiten geprüft'), findsOneWidget);

      await tester.pump(const Duration(seconds: 2));
      await tester.pump();
      expect(find.text('100 %'), findsOneWidget);

      final calls = api.getCalls;
      await tester.pump(const Duration(seconds: 6));
      expect(api.getCalls, calls,
          reason: 'polling must stop after the scan finished');
    });

    // Spec: F1 — failed scans show the German reason.
    testWidgets('failed scan shows the reason', (tester) async {
      await openResult(
          tester,
          FakeApi(statuses: [
            statusOf('new123', 'failed',
                errorDe: 'Die Startseite konnte nicht geladen werden.')
          ]));
      await tester.pumpAndSettle();
      expect(find.text('Die Prüfung ist fehlgeschlagen.'), findsOneWidget);
      expect(find.text('Die Startseite konnte nicht geladen werden.'),
          findsOneWidget);
    });

    // Spec: F11 — unknown or expired id.
    testWidgets('unknown scan shows the 404 message', (tester) async {
      await openResult(
        tester,
        FakeApi(
            statuses: [statusOf('x', 'queued')],
            getError: const ApiException('Diese Prüfung gibt es nicht (mehr).',
                status: 404)),
      );
      await tester.pumpAndSettle();
      expect(find.text('Diese Prüfung gibt es nicht (mehr).'), findsOneWidget);
    });
  });
}
