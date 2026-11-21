import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../strings.dart';

/// Finding detail (F8): explanation, location, crop and the copyable fix.
class FindingPage extends StatefulWidget {
  const FindingPage(
      {super.key,
      required this.api,
      required this.scanId,
      required this.findingId});

  final ApiClient api;
  final String scanId;
  final String findingId;

  @override
  State<FindingPage> createState() => _FindingPageState();
}

class _FindingPageState extends State<FindingPage> {
  Finding? _finding;
  String? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final f = await widget.api.getFindings(widget.scanId);
      if (!mounted) return;
      final hit = f.findings.where((x) => x.id == widget.findingId);
      setState(() {
        _finding = hit.isEmpty ? null : hit.first;
        _error = hit.isEmpty ? 'Diese Fundstelle gibt es nicht.' : null;
        _loading = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.titleDe;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Fundstelle')),
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
    final back = Align(
      alignment: Alignment.centerLeft,
      child: TextButton.icon(
        onPressed: () => context.go('/r/${widget.scanId}'),
        icon: const Icon(Icons.arrow_back),
        label: const Text('Zurück zum Ergebnis'),
      ),
    );
    if (_loading) {
      return [back, const Center(child: CircularProgressIndicator())];
    }
    final f = _finding;
    if (f == null) {
      return [
        back,
        Text(_error ?? 'Diese Fundstelle gibt es nicht.',
            style: TextStyle(color: theme.colorScheme.error)),
      ];
    }
    const mono = TextStyle(fontFamily: 'monospace');
    return [
      back,
      const SizedBox(height: 8),
      Text(
          '${severityLabelDe(f.severity)} · ${f.ruleId} ${ruleNameDe(f.ruleId)}',
          style: theme.textTheme.titleLarge),
      const SizedBox(height: 8),
      if (f.summaryDe.isNotEmpty) Text(f.summaryDe),
      if (f.explanationDe.isNotEmpty) ...[
        const SizedBox(height: 8),
        Text(f.explanationDe),
      ],
      const SizedBox(height: 16),
      Text('Seite', style: theme.textTheme.labelLarge),
      SelectableText(f.url),
      if (f.selector.isNotEmpty) ...[
        const SizedBox(height: 8),
        Text('Element', style: theme.textTheme.labelLarge),
        SelectableText(f.selector, style: mono),
      ],
      if (f.snippet.isNotEmpty) ...[
        const SizedBox(height: 4),
        SelectableText(f.snippet, style: mono),
      ],
      if (f.hasCrop) ...[
        const SizedBox(height: 16),
        Image.network(
          widget.api.cropUri(widget.scanId, f.id).toString(),
          semanticLabel: 'Ausschnitt der Seite mit der Fundstelle',
          errorBuilder: (_, __, ___) => const SizedBox.shrink(),
        ),
      ],
      if (f.fix != null) ...[
        const SizedBox(height: 24),
        _FixCard(fix: f.fix!),
      ],
    ];
  }
}

class _FixCard extends StatelessWidget {
  const _FixCard({required this.fix});

  final FindingFix fix;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final code = fix.code;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text('So können Sie es beheben',
                    style: theme.textTheme.titleMedium),
                Chip(
                    label:
                        Text(fix.origin == 'ai' ? 'KI-Vorschlag' : 'Vorlage')),
              ],
            ),
            const SizedBox(height: 8),
            Text(fix.textDe),
            if (code != null && code.isNotEmpty) ...[
              const SizedBox(height: 12),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(12),
                color: theme.colorScheme.surfaceContainerHighest,
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: SelectableText(code,
                      style: const TextStyle(fontFamily: 'monospace')),
                ),
              ),
              const SizedBox(height: 8),
              FilledButton.tonalIcon(
                onPressed: () async {
                  final messenger = ScaffoldMessenger.of(context);
                  await Clipboard.setData(ClipboardData(text: code));
                  messenger
                      .showSnackBar(const SnackBar(content: Text('Kopiert')));
                },
                icon: const Icon(Icons.copy),
                label: const Text('Kopieren'),
              ),
            ],
            if (fix.noteDe != null && fix.noteDe!.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(fix.noteDe!, style: theme.textTheme.bodySmall),
            ],
          ],
        ),
      ),
    );
  }
}
