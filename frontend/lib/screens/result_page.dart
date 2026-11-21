import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api/api_client.dart';
import '../strings.dart';

/// Starts a file download; returns false if it could not be started.
typedef Downloader = Future<bool> Function(Uri uri);

/// Default downloader: the API answers with `Content-Disposition: attachment`, so opening the URL in the
/// same tab downloads the file and the page stays where it is. The visual HTML report without `download`
/// is shown, so it opens in a new tab (F14).
Future<bool> launchDownload(Uri uri) => launchUrl(uri,
    webOnlyWindowName: isReportView(uri) ? '_blank' : '_self');

/// True for the HTML report URL that shows the report instead of downloading it.
bool isReportView(Uri uri) =>
    uri.path.endsWith('/report.html') &&
    !uri.queryParameters.containsKey('download');

/// Progress and report for one scan. Spec: F7 (score, severity, list), F10/F13/F14 (report and downloads),
/// F11 (link, 1 hour), N3 (disclaimer), ADR-6 (nothing is stored).
class ResultPage extends StatefulWidget {
  const ResultPage({
    super.key,
    required this.api,
    required this.scanId,
    this.pollInterval = const Duration(seconds: 2),
    this.onDownload = launchDownload,
  });

  final ApiClient api;
  final String scanId;
  final Duration pollInterval;
  final Downloader onDownload;

  @override
  State<ResultPage> createState() => _ResultPageState();
}

class _ResultPageState extends State<ResultPage> {
  ScanStatus? _status;
  ScanFindings? _findings;
  String? _error;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final s = await widget.api.getScan(widget.scanId);
      ScanFindings? f;
      if (s.hasResults) {
        f = await widget.api.getFindings(widget.scanId);
      }
      if (!mounted) return;
      setState(() {
        _status = s;
        _findings = f ?? _findings;
        _error = null;
      });
      if (s.isFinished) {
        _timer?.cancel();
      } else {
        _timer ??= Timer.periodic(widget.pollInterval, (_) => _load());
      }
    } on ApiException catch (e) {
      if (!mounted) return;
      // 404 is final; other errors (network) keep polling.
      setState(() => _error = e.titleDe);
      if (e.status == 404) {
        _timer?.cancel();
      } else {
        _timer ??= Timer.periodic(widget.pollInterval, (_) => _load());
      }
    }
  }

  /// Opens the visual report (view) or downloads the report as [ext]. The HTML file download needs
  /// `?download=1`, otherwise the server shows it (F14, specs/11-report.md).
  Future<void> _download(BuildContext context, String ext,
      {bool view = false}) async {
    final messenger = ScaffoldMessenger.of(context);
    var uri = widget.api.reportUri(widget.scanId, ext);
    if (ext == 'html' && !view) {
      uri = uri.replace(queryParameters: const {'download': '1'});
    }
    final ok = await widget.onDownload(uri);
    if (!ok) {
      messenger.showSnackBar(const SnackBar(
          content: Text('Der Download konnte nicht gestartet werden.')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Prüfergebnis'),
        actions: [
          TextButton(
              onPressed: () => context.go('/'),
              child: const Text('Neue Prüfung')),
        ],
      ),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: ListView(
              padding: const EdgeInsets.all(24), children: _content(context)),
        ),
      ),
    );
  }

  List<Widget> _content(BuildContext context) {
    final theme = Theme.of(context);
    final s = _status;

    if (s == null) {
      if (_error != null) {
        return [
          Text(_error!, style: TextStyle(color: theme.colorScheme.error))
        ];
      }
      return const [Center(child: CircularProgressIndicator())];
    }

    if (s.status == 'failed') {
      return [
        Text('Die Prüfung ist fehlgeschlagen.',
            style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        Text(s.errorDe ?? 'Unbekannter Fehler.'),
        const SizedBox(height: 24),
        Text(s.disclaimerDe ?? disclaimerDe, style: theme.textTheme.bodySmall),
      ];
    }

    if (!s.hasResults) {
      final waiting = s.status == 'queued';
      return [
        Text(
            waiting
                ? 'Ihre Prüfung wartet in der Schlange …'
                : 'Ihr Shop wird geprüft …',
            style: theme.textTheme.titleLarge),
        const SizedBox(height: 16),
        Semantics(
          label: 'Fortschritt der Prüfung',
          child: LinearProgressIndicator(
              value: s.pagesTotal > 0 ? s.pagesDone / s.pagesTotal : null),
        ),
        if (s.pagesTotal > 0) ...[
          const SizedBox(height: 8),
          Text('${s.pagesDone} von ${s.pagesTotal} Seiten geprüft'),
        ],
      ];
    }

    final f = _findings;
    return [
      _ScoreCard(status: s),
      const SizedBox(height: 8),
      if (s.status == 'incomplete')
        const Text(
            'Die Prüfung wurde wegen des Zeitlimits nicht vollständig durchgeführt.'),
      const SizedBox(height: 8),
      _DownloadCard(
        status: s,
        onDownload: (ext) => _download(context, ext),
        onView: () => _download(context, 'html', view: true),
      ),
      const SizedBox(height: 24),
      if (f != null) ...[
        Text('Prüfregeln', style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        if (f.ruleResults.isEmpty)
          const Text(
              'Für die gewählten Bereiche wurden noch keine Regeln ausgeführt.'),
        for (final r in f.ruleResults) _RuleTile(result: r),
        const SizedBox(height: 24),
        Text('Gefundene Probleme (${f.findings.length})',
            style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        if (f.findings.isEmpty)
          const Text('Es wurden keine Probleme gefunden.'),
        for (final x in f.findings) _FindingTile(finding: x),
      ],
      const SizedBox(height: 32),
      Text(s.disclaimerDe ?? disclaimerDe, style: theme.textTheme.bodySmall),
    ];
  }
}

/// Tells the user that nothing is stored and offers the report and downloads (ADR-6, F10, F13, F14, F11).
class _DownloadCard extends StatelessWidget {
  const _DownloadCard(
      {required this.status, required this.onDownload, required this.onView});

  final ScanStatus status;
  final void Function(String ext) onDownload;
  final VoidCallback onView;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final until = status.expiresAt == null
        ? 'eine Stunde lang'
        : 'bis ${formatTimeDe(status.expiresAt!)} Uhr';
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Bericht ansehen und herunterladen',
                style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              'Dieses Ergebnis wird nicht gespeichert. Es ist nur $until über '
              'diesen Link abrufbar und wird danach gelöscht. Laden Sie den '
              'Bericht herunter, um ihn zu behalten. Im Bericht sind die '
              'Fundstellen in Screenshots der Seiten markiert.',
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 12,
              runSpacing: 8,
              children: [
                FilledButton.icon(
                  onPressed: onView,
                  icon: const Icon(Icons.open_in_new),
                  label: const Text('Bericht ansehen'),
                ),
                OutlinedButton.icon(
                  onPressed: () => onDownload('pdf'),
                  icon: const Icon(Icons.picture_as_pdf_outlined),
                  label: const Text('Bericht als PDF'),
                ),
                OutlinedButton.icon(
                  onPressed: () => onDownload('html'),
                  icon: const Icon(Icons.code),
                  label: const Text('Bericht als HTML'),
                ),
                OutlinedButton.icon(
                  onPressed: () => onDownload('json'),
                  icon: const Icon(Icons.data_object),
                  label: const Text('Daten als JSON'),
                ),
                TextButton.icon(
                  onPressed: () async {
                    final messenger = ScaffoldMessenger.of(context);
                    await Clipboard.setData(
                        ClipboardData(text: Uri.base.toString()));
                    messenger.showSnackBar(
                        const SnackBar(content: Text('Link kopiert.')));
                  },
                  icon: const Icon(Icons.link),
                  label: const Text('Link kopieren'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _ScoreCard extends StatelessWidget {
  const _ScoreCard({required this.status});

  final ScanStatus status;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final score = status.score;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Semantics(
          container: true,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Anteil bestandener automatischer Prüfungen',
                  style: theme.textTheme.titleMedium),
              const SizedBox(height: 8),
              Text(score == null ? 'nicht berechenbar' : '$score %',
                  style: theme.textTheme.displaySmall),
              const SizedBox(height: 4),
              const Text(
                  'Das ist keine Aussage über vollständige Konformität.'),
            ],
          ),
        ),
      ),
    );
  }
}

class _RuleTile extends StatelessWidget {
  const _RuleTile({required this.result});

  final RuleResult result;

  @override
  Widget build(BuildContext context) {
    final (icon, color) = switch (result.state) {
      'passed' => (Icons.check_circle_outline, Colors.green.shade800),
      'failed' => (Icons.error_outline, Theme.of(context).colorScheme.error),
      _ => (Icons.help_outline, Theme.of(context).colorScheme.onSurfaceVariant),
    };
    final label = stateLabelDe(result.state);
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: Icon(icon, color: color, semanticLabel: label),
      title: Text('${result.ruleId} · ${ruleNameDe(result.ruleId)}'),
      subtitle: Text(
        result.state == 'failed' && result.pagesAffected > 0
            ? '$label · ${result.pagesAffected} Seite(n) betroffen'
            : label,
      ),
    );
  }
}

class _FindingTile extends StatelessWidget {
  const _FindingTile({required this.finding});

  final Finding finding;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      child: ListTile(
        title: Text(
            '${severityLabelDe(finding.severity)} · ${finding.ruleId} ${ruleNameDe(finding.ruleId)}'),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(finding.summaryDe),
            if (finding.url.isNotEmpty)
              Text(finding.url, style: theme.textTheme.bodySmall),
          ],
        ),
      ),
    );
  }
}
