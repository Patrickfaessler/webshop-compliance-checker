import 'package:compliance_checker/main.dart';
import 'package:compliance_checker/strings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_api.dart';

// Spec: N3
void main() {
  testWidgets('start page shows the legal disclaimer', (tester) async {
    // the disclaimer sits below the form; use a view tall enough to build it
    tester.view.physicalSize = const Size(900, 1600);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(App(api: FakeApi()));
    await tester.pumpAndSettle();
    expect(find.text(disclaimerDe), findsOneWidget);
  });
}
