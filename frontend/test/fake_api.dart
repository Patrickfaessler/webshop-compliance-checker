import 'package:compliance_checker/api/api_client.dart';

/// In-memory ApiClient for widget tests.
class FakeApi implements ApiClient {
  FakeApi(
      {this.createError,
      List<ScanStatus>? statuses,
      this.findings,
      this.getError})
      : _statuses = statuses ?? [];

  final ApiException? createError;
  final ApiException? getError;
  final List<ScanStatus> _statuses; // returned in order; the last one repeats
  final ScanFindings? findings;

  String? createdUrl;
  List<String>? createdAreas;
  int getCalls = 0;

  @override
  Future<ScanStatus> createScan(String url, List<String> areas) async {
    if (createError != null) throw createError!;
    createdUrl = url;
    createdAreas = areas;
    return statusOf('new123', 'queued');
  }

  @override
  Future<ScanStatus> getScan(String id) async {
    getCalls++;
    if (getError != null) throw getError!;
    final i =
        getCalls - 1 < _statuses.length ? getCalls - 1 : _statuses.length - 1;
    return _statuses[i];
  }

  @override
  Future<ScanFindings> getFindings(String id) async =>
      findings ?? const ScanFindings(ruleResults: [], findings: []);
}

ScanStatus statusOf(
  String id,
  String status, {
  int? score,
  int done = 0,
  int total = 0,
  String? errorDe,
  DateTime? expiresAt,
}) =>
    ScanStatus(
      id: id,
      status: status,
      pagesDone: done,
      pagesTotal: total,
      score: score,
      errorDe: errorDe,
      disclaimerDe: 'DISCLAIMER-VOM-SERVER',
      expiresAt: expiresAt,
    );
