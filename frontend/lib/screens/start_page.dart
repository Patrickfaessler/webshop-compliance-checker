import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../strings.dart';

/// Adds https:// when the user typed a bare host name. Spec: F1.
String normalizeShopUrl(String input) {
  final t = input.trim();
  if (t.isEmpty) return t;
  return t.contains('://') ? t : 'https://$t';
}

/// URL entry. Spec: F1 (no login), N3 (disclaimer), N7 (labelled controls).
class StartPage extends StatefulWidget {
  const StartPage({super.key, required this.api});

  final ApiClient api;

  @override
  State<StartPage> createState() => _StartPageState();
}

class _StartPageState extends State<StartPage> {
  final _controller = TextEditingController();
  final _presses = TextEditingController(text: '200'); // A5-8
  bool _keyboardAllPages = false; // A5-6
  bool _accessibility = true;
  bool _legal = true;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _controller.dispose();
    _presses.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final url = normalizeShopUrl(_controller.text);
    if (url.isEmpty) {
      setState(() => _error = 'Bitte die Adresse Ihres Shops eingeben.');
      return;
    }
    final areas = [if (_accessibility) 'accessibility', if (_legal) 'legal'];
    if (areas.isEmpty) {
      setState(() => _error = 'Bitte mindestens einen Prüfbereich auswählen.');
      return;
    }
    final presses = int.tryParse(_presses.text.trim());
    if (presses == null || presses < 50 || presses > 1000) {
      setState(() => _error = 'Bitte eine Zahl von 50 bis 1000 eingeben.');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final scan = await widget.api.createScan(url, areas,
          keyboardAllPages: _accessibility && _keyboardAllPages,
          keyboardMaxPresses: presses);
      if (!mounted) return;
      context.go('/r/${scan.id}');
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.titleDe);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Webshop-Check')),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Text('Ist Ihr Webshop barrierefrei und rechtlich in Ordnung?',
                  style: theme.textTheme.headlineSmall),
              const SizedBox(height: 8),
              const Text(
                'Adresse eingeben, prüfen lassen, verständliche Erklärungen und Lösungsvorschläge bekommen. '
                'Ohne Anmeldung.',
              ),
              const SizedBox(height: 24),
              TextField(
                controller: _controller,
                keyboardType: TextInputType.url,
                autofillHints: const [AutofillHints.url],
                textInputAction: TextInputAction.go,
                onSubmitted: (_) => _busy ? null : _submit(),
                decoration: const InputDecoration(
                  labelText: 'Adresse Ihres Shops',
                  hintText: 'www.mein-shop.at',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 8),
              CheckboxListTile(
                value: _accessibility,
                onChanged: _busy
                    ? null
                    : (v) => setState(() => _accessibility = v ?? false),
                title: const Text('Barrierefreiheit (WCAG 2.1 AA)'),
                controlAffinity: ListTileControlAffinity.leading,
                contentPadding: EdgeInsets.zero,
              ),
              // A5 options (specs/rules/A5-keyboard.md), only with the accessibility area
              Padding(
                padding: const EdgeInsets.only(left: 32),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    CheckboxListTile(
                      value: _accessibility && _keyboardAllPages,
                      onChanged: _busy || !_accessibility
                          ? null
                          : (v) =>
                              setState(() => _keyboardAllPages = v ?? false),
                      title: const Text(
                          'Gründliche Tastaturprüfung auf allen Seiten (dauert bis zu 10 Minuten)'),
                      controlAffinity: ListTileControlAffinity.leading,
                      contentPadding: EdgeInsets.zero,
                    ),
                    SizedBox(
                      width: 260,
                      child: TextField(
                        controller: _presses,
                        enabled: !_busy && _accessibility,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(
                          labelText: 'Tab-Drücke pro Seite',
                          helperText:
                              '50–1000; mehr = gründlicher, aber langsamer',
                          border: OutlineInputBorder(),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 8),
              CheckboxListTile(
                value: _legal,
                onChanged:
                    _busy ? null : (v) => setState(() => _legal = v ?? false),
                title: const Text('Rechtliche Pflichtangaben (EU-Ebene)'),
                controlAffinity: ListTileControlAffinity.leading,
                contentPadding: EdgeInsets.zero,
              ),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Semantics(
                    liveRegion: true,
                    child: Text(_error!,
                        style: TextStyle(color: theme.colorScheme.error)),
                  ),
                ),
              const SizedBox(height: 8),
              Align(
                alignment: Alignment.centerLeft,
                child: FilledButton(
                  onPressed: _busy ? null : _submit,
                  child: Text(_busy ? 'Wird gestartet …' : 'Jetzt prüfen'),
                ),
              ),
              const SizedBox(height: 32),
              Text(disclaimerDe, style: theme.textTheme.bodySmall),
            ],
          ),
        ),
      ),
    );
  }
}
