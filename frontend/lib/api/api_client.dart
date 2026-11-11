/// Client for the REST contract in specs/08-api.yaml.
library;

import 'dart:convert';

import 'package:http/http.dart' as http;

class ScanStatus {
  const ScanStatus({
    required this.id,
    required this.status,
    required this.pagesDone,
    required this.pagesTotal,
    required this.score,
    required this.errorDe,
    required this.disclaimerDe,
    required this.expiresAt,
  });

  final String id;
  final String status; // queued | running | done | incomplete | failed
  final int pagesDone;
  final int pagesTotal;
  final int? score;
  final String? errorDe;
  final String? disclaimerDe;
  final DateTime? expiresAt;

  bool get isFinished =>
      status == 'done' || status == 'incomplete' || status == 'failed';
  bool get hasResults => status == 'done' || status == 'incomplete';

  factory ScanStatus.fromJson(Map<String, dynamic> j) => ScanStatus(
        id: j['id'] as String,
        status: j['status'] as String,
        pagesDone: (j['pagesDone'] as num?)?.toInt() ?? 0,
        pagesTotal: (j['pagesTotal'] as num?)?.toInt() ?? 0,
        score: (j['score'] as num?)?.toInt(),
        errorDe: j['errorDe'] as String?,
        disclaimerDe: j['disclaimerDe'] as String?,
        expiresAt: j['expiresAt'] == null
            ? null
            : DateTime.tryParse(j['expiresAt'] as String),
      );
}

class RuleResult {
  const RuleResult(
      {required this.ruleId,
      required this.state,
      required this.decidedBy,
      required this.pagesAffected});

  final String ruleId;
  final String state; // passed | failed | not_applicable | not_assessable
  final String decidedBy;
  final int pagesAffected;

  factory RuleResult.fromJson(Map<String, dynamic> j) => RuleResult(
        ruleId: j['ruleId'] as String,
        state: j['state'] as String,
        decidedBy: (j['decidedBy'] as String?) ?? '',
        pagesAffected: (j['pagesAffected'] as num?)?.toInt() ?? 0,
      );
}

class Finding {
  const Finding(
      {required this.id,
      required this.ruleId,
      required this.severity,
      required this.url,
      required this.summaryDe});

  final String id;
  final String ruleId;
  final String severity; // critical | warning | notice
  final String url;
  final String summaryDe;

  factory Finding.fromJson(Map<String, dynamic> j) => Finding(
        id: j['id'] as String,
        ruleId: j['ruleId'] as String,
        severity: j['severity'] as String,
        url: (j['url'] as String?) ?? '',
        summaryDe: (j['summaryDe'] as String?) ?? '',
      );
}

class ScanFindings {
  const ScanFindings({required this.ruleResults, required this.findings});

  final List<RuleResult> ruleResults;
  final List<Finding> findings;

  factory ScanFindings.fromJson(Map<String, dynamic> j) => ScanFindings(
        ruleResults: [
          for (final r in (j['ruleResults'] as List? ?? const []))
            RuleResult.fromJson(r as Map<String, dynamic>)
        ],
        findings: [
          for (final f in (j['findings'] as List? ?? const []))
            Finding.fromJson(f as Map<String, dynamic>)
        ],
      );
}

/// A problem reported by the API; [titleDe] is a German user message.
class ApiException implements Exception {
  const ApiException(this.titleDe, {this.status});

  final String titleDe;
  final int? status;

  @override
  String toString() => 'ApiException($status): $titleDe';
}

abstract class ApiClient {
  Future<ScanStatus> createScan(String url, List<String> areas);
  Future<ScanStatus> getScan(String id);
  Future<ScanFindings> getFindings(String id);

  /// Download URL of the finished report; [ext] is `pdf` or `json` (F10, F13).
  Uri reportUri(String id, String ext);
}

const _apiBase =
    String.fromEnvironment('API_BASE', defaultValue: 'http://localhost:8080');

/// An empty API_BASE means "same origin": the server deployment serves the app and /api on one host (DEP-2).
String resolveApiBase(String configured, Uri page) =>
    configured.isEmpty ? page.origin : configured;

class HttpApiClient implements ApiClient {
  HttpApiClient({http.Client? client, String? baseUrl})
      : _http = client ?? http.Client(),
        _base = baseUrl ?? resolveApiBase(_apiBase, Uri.base);

  final http.Client _http;
  final String _base;

  @override
  Future<ScanStatus> createScan(String url, List<String> areas) async {
    final json = await _send(() => _http.post(
          Uri.parse('$_base/api/scans'),
          headers: {'Content-Type': 'application/json'},
          body: jsonEncode({'url': url, 'areas': areas}),
        ));
    return ScanStatus.fromJson(json);
  }

  @override
  Future<ScanStatus> getScan(String id) async =>
      ScanStatus.fromJson(await _send(() =>
          _http.get(Uri.parse('$_base/api/scans/${Uri.encodeComponent(id)}'))));

  @override
  Uri reportUri(String id, String ext) =>
      Uri.parse('$_base/api/scans/${Uri.encodeComponent(id)}/report.$ext');

  @override
  Future<ScanFindings> getFindings(String id) async =>
      ScanFindings.fromJson(await _send(() => _http.get(
          Uri.parse('$_base/api/scans/${Uri.encodeComponent(id)}/findings'))));

  Future<Map<String, dynamic>> _send(
      Future<http.Response> Function() call) async {
    final http.Response resp;
    try {
      resp = await call();
    } catch (_) {
      throw const ApiException(
          'Der Server ist nicht erreichbar. Bitte später erneut versuchen.');
    }
    Map<String, dynamic>? body;
    try {
      body = jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;
    } catch (_) {
      body = null;
    }
    if (resp.statusCode >= 200 && resp.statusCode < 300 && body != null) {
      return body;
    }
    throw ApiException(
      (body?['title'] as String?) ??
          'Unerwarteter Fehler (${resp.statusCode}).',
      status: resp.statusCode,
    );
  }
}
