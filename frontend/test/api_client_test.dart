import 'dart:convert';

import 'package:compliance_checker/api/api_client.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

HttpApiClient clientFor(http.Response Function(http.Request) handler) =>
    HttpApiClient(
        client: MockClient((r) async => handler(r)),
        baseUrl: 'http://api.test');

http.Response json(Object body, [int code = 200]) =>
    http.Response.bytes(utf8.encode(jsonEncode(body)), code,
        headers: {'content-type': 'application/json; charset=utf-8'});

// Spec: specs/08-api.yaml
void main() {
  // Spec: DEP-2 — an empty API_BASE uses the page's own origin.
  test('empty API_BASE means same origin', () {
    final page = Uri.parse('https://check.example.org/r/abc?x=1');
    expect(resolveApiBase('', page), 'https://check.example.org');
    expect(resolveApiBase('http://localhost:8080', page), 'http://localhost:8080');
  });

  test('createScan posts url and areas and parses the status', () async {
    late http.Request seen;
    final api = clientFor((r) {
      seen = r;
      return json({
        'id': 'abc',
        'status': 'queued',
        'pagesDone': 0,
        'pagesTotal': 0,
        'score': null,
        'expiresAt': '2026-12-05T12:00:00Z',
        'disclaimerDe': 'x',
      }, 202);
    });
    final s = await api.createScan('https://shop.at', ['legal']);
    expect(seen.method, 'POST');
    expect(seen.url.toString(), 'http://api.test/api/scans');
    expect(jsonDecode(seen.body), {
      'url': 'https://shop.at',
      'areas': ['legal']
    });
    expect(s.id, 'abc');
    expect(s.score, isNull);
    expect(s.expiresAt, DateTime.utc(2026, 12, 5, 12));
    expect(s.isFinished, isFalse);
  });

  test('getFindings parses rule results and findings (umlauts intact)',
      () async {
    final api = clientFor((_) => json({
          'ruleResults': [
            {
              'ruleId': 'A6',
              'state': 'failed',
              'decidedBy': 'structural',
              'pagesAffected': 1
            },
          ],
          'findings': [
            {
              'id': 'f1',
              'ruleId': 'A6',
              'severity': 'notice',
              'url': 'https://shop.at/',
              'summaryDe': 'Überschrift fehlt'
            },
          ],
        }));
    final f = await api.getFindings('abc');
    expect(f.ruleResults.single.state, 'failed');
    expect(f.findings.single.summaryDe, 'Überschrift fehlt');
  });

  test('problem+json title becomes ApiException with status', () async {
    final api = clientFor(
        (_) => json({'title': 'Zu viele Prüfungen.', 'status': 429}, 429));
    expect(
      () => api.getScan('abc'),
      throwsA(isA<ApiException>()
          .having((e) => e.titleDe, 'title', 'Zu viele Prüfungen.')
          .having((e) => e.status, 'status', 429)),
    );
  });

  test('network failure becomes a German ApiException', () async {
    final api = HttpApiClient(
        client: MockClient((_) async => throw Exception('offline')),
        baseUrl: 'http://api.test');
    expect(
        () => api.getScan('abc'),
        throwsA(isA<ApiException>()
            .having((e) => e.titleDe, 'title', contains('nicht erreichbar'))));
  });

  test('scan id is URL-encoded in the path', () async {
    late Uri seen;
    final api = clientFor((r) {
      seen = r.url;
      return json({'id': 'x', 'status': 'done'});
    });
    await api.getScan('a/b');
    expect(seen.toString(), 'http://api.test/api/scans/a%2Fb');
  });
}
