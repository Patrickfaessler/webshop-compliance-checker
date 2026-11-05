import 'package:compliance_checker/main.dart';
import 'package:compliance_checker/strings.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_api.dart';

// Spec: N3
void main() {
  testWidgets('start page shows the legal disclaimer', (tester) async {
    await tester.pumpWidget(App(api: FakeApi()));
    await tester.pumpAndSettle();
    expect(find.text(disclaimerDe), findsOneWidget);
  });
}
