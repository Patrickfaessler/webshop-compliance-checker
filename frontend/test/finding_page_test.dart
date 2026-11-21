import 'package:compliance_checker/api/api_client.dart';
import 'package:compliance_checker/main.dart';
import 'package:compliance_checker/screens/start_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'fake_api.dart';

const _fix = FindingFix(
    origin: 'template',
    textDe: 'Gib dem Feld eine sichtbare Beschriftung.',
    code: '<label for="plz">PLZ</label>',
    noteDe: 'Beschriftungstext bitte anpassen.');

Finding _finding({bool hasCrop = false, FindingFix? fix = _fix}) => Finding(
      id: 'f1',
      ruleId: 'A3',
      severity: 'critical',
      url: 'https://shop.at/kasse.html',
      summaryDe: 'Formularfeld ohne Beschriftung',
      selector: '#plz',
      snippet: '<input id="plz" placeholder="PLZ">',
      explanationDe: 'Dieses Eingabefeld hat keine Beschriftung.',
      hasCrop: hasCrop,
      fix: fix,
    );

FakeApi _api({Finding? f}) => FakeApi(
      statuses: [statusOf('s1', 'done', score: 50)],
      findings:
          ScanFindings(ruleResults: const [], findings: [f ?? _finding()]),
    );

Future<void> _open(WidgetTester tester, FakeApi api, String location,
    {Size size = const Size(900, 1600)}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(App(api: api));
  await tester.pumpAndSettle();
  GoRouter.of(tester.element(find.byType(StartPage))).go(location);
  await tester.pumpAndSettle();
}

void main() {
  // Spec: F8 — the finding tile opens the detail page with explanation, location and the copyable fix.
  testWidgets('finding detail page', (tester) async {
    String? copied;
    tester.binding.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        copied = (call.arguments as Map)['text'] as String;
      }
      return null;
    });
    addTearDown(() => tester.binding.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null));
    final api = _api();
    await _open(tester, api, '/r/s1');
    await tester.tap(find.text('Formularfeld ohne Beschriftung'));
    await tester.pumpAndSettle();
    for (final t in [
      'Dieses Eingabefeld hat keine Beschriftung.',
      'https://shop.at/kasse.html',
      '#plz',
      '<input id="plz" placeholder="PLZ">',
      'Gib dem Feld eine sichtbare Beschriftung.',
      '<label for="plz">PLZ</label>',
      'Beschriftungstext bitte anpassen.',
      'Vorlage',
    ]) {
      expect(find.text(t), findsOneWidget, reason: t);
    }
    await tester.tap(find.text('Kopieren'));
    await tester.pump();
    expect(copied, '<label for="plz">PLZ</label>');
    expect(find.text('Kopiert'), findsOneWidget);
    await tester.pumpAndSettle(const Duration(seconds: 5));
    await tester.tap(find.text('Zurück zum Ergebnis'));
    await tester.pumpAndSettle();
    expect(find.text('Prüfregeln'), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });

  // Spec: F8 — the crop is shown only when the API has one (Review Focus 3).
  testWidgets('crop image only with hasCrop', (tester) async {
    await _open(tester, _api(), '/r/s1/f/f1');
    expect(find.byType(Image), findsNothing);
    await tester.pumpWidget(const SizedBox());
    await _open(tester, _api(f: _finding(hasCrop: true)), '/r/s1/f/f1');
    final img = tester.widget<Image>(find.byType(Image));
    expect((img.image as NetworkImage).url,
        'http://api.test/api/scans/s1/findings/f1/crop.jpg');
    await tester.pumpWidget(const SizedBox());
  });

  // Spec: F8 — a fix without code shows no code box and no copy button.
  testWidgets('fix without code', (tester) async {
    await _open(
        tester,
        _api(
            f: _finding(
                fix: const FindingFix(
                    origin: 'template', textDe: 'Ändere <h3> zu <h2>.'))),
        '/r/s1/f/f1');
    expect(find.text('Ändere <h3> zu <h2>.'), findsOneWidget);
    expect(find.text('Kopieren'), findsNothing);
    await tester.pumpWidget(const SizedBox());
  });

  // Spec: F8 — direct link to an unknown finding or an expired scan (Review Focus 4).
  testWidgets('unknown finding and expired scan', (tester) async {
    await _open(tester, _api(), '/r/s1/f/missing');
    expect(find.text('Diese Fundstelle gibt es nicht.'), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
    final gone = _api()
      ..findingsError = const ApiException(
          'Diese Prüfung gibt es nicht (mehr).',
          status: 404);
    await _open(tester, gone, '/r/s1/f/f1');
    expect(find.text('Diese Prüfung gibt es nicht (mehr).'), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });

  // Spec: F8 — the detail page fits a 360 px phone with a long code line (Review Focus 5).
  testWidgets('detail page fits 360 px', (tester) async {
    final long = FindingFix(origin: 'template', textDe: 't', code: 'x' * 300);
    await _open(tester, _api(f: _finding(fix: long)), '/r/s1/f/f1',
        size: const Size(360, 800));
    expect(tester.takeException(), isNull);
    expect(find.text('x' * 300), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });
}
