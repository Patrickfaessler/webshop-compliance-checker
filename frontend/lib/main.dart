import 'package:flutter/material.dart';
import 'package:flutter_web_plugins/url_strategy.dart';
import 'package:go_router/go_router.dart';

import 'api/api_client.dart';
import 'screens/result_page.dart';
import 'screens/start_page.dart';

void main() {
  usePathUrlStrategy(); // clean /r/{id} links (F11)
  runApp(App(api: HttpApiClient()));
}

class App extends StatefulWidget {
  const App({super.key, required this.api});

  final ApiClient api;

  @override
  State<App> createState() => _AppState();
}

class _AppState extends State<App> {
  late final GoRouter _router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, __) => StartPage(api: widget.api)),
      GoRoute(
        path: '/r/:id',
        builder: (_, state) =>
            ResultPage(api: widget.api, scanId: state.pathParameters['id']!),
      ),
    ],
  );

  @override
  void dispose() {
    _router.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      title: 'Webshop-Check',
      locale: const Locale('de'),
      theme: ThemeData(colorSchemeSeed: Colors.indigo, useMaterial3: true),
      routerConfig: _router,
    );
  }
}
