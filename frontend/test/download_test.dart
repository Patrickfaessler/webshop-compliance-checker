import 'package:compliance_checker/api/api_client.dart';
import 'package:compliance_checker/main.dart';
import 'package:compliance_checker/screens/result_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_api.dart';

Future<void> openFinished(
    WidgetTester tester, FakeApi api, Downloader dl) async {
  tester.view.physicalSize = const Size(900, 3000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(App(api: api, onDownload: dl));
  await tester.pumpAndSettle();
  await tester.enterText(find.byType(TextField), 'shop.at');
  await tester.tap(find.text('Jetzt prüfen'));
  await tester.pumpAndSettle();
}

void main() {
  // Spec: F10, F13, ADR-6 — the buttons request the PDF and JSON report of this scan.
  testWidgets('download buttons open the report URLs', (tester) async {
    final opened = <Uri>[];
    final api = FakeApi(statuses: [statusOf('new123', 'done', score: 100)]);
    await openFinished(tester, api, (uri) async {
      opened.add(uri);
      return true;
    });

    await tester.tap(find.text('Bericht als PDF'));
    await tester.pump();
    await tester.tap(find.text('Daten als JSON'));
    await tester.pump();
    expect(opened.map((u) => u.toString()), [
      'http://api.test/api/scans/new123/report.pdf',
      'http://api.test/api/scans/new123/report.json',
    ]);
  });

  // Spec: F10 — the user learns when a download cannot start.
  testWidgets('failed download shows a message', (tester) async {
    final api = FakeApi(statuses: [statusOf('new123', 'done', score: 100)]);
    await openFinished(tester, api, (_) async => false);
    await tester.tap(find.text('Bericht als PDF'));
    await tester.pump();
    expect(find.text('Der Download konnte nicht gestartet werden.'),
        findsOneWidget);
  });

  // Spec: F10, F13 — no downloads while the scan is still running.
  testWidgets('no download buttons while running', (tester) async {
    final api =
        FakeApi(statuses: [statusOf('new123', 'running', done: 1, total: 3)]);
    await openFinished(tester, api, (_) async => true);
    expect(find.text('Bericht als PDF'), findsNothing);
    await tester.pumpWidget(const SizedBox()); // stop polling
  });

  // Spec: F10, F13 — URLs are built from the API base and encode the id.
  test('HttpApiClient.reportUri', () {
    final c = HttpApiClient(baseUrl: 'http://localhost:8080');
    expect(c.reportUri('a/b', 'pdf').toString(),
        'http://localhost:8080/api/scans/a%2Fb/report.pdf');
  });
}
