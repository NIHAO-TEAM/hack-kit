# Спецификации

Текущее поведение системы. Папка на возможность (`specs/<capability>/spec.md`): `auth`, `organization`, `deliveries`, `orders`… Внутри — требования (SHALL/MUST) и сценарии WHEN/THEN.

Руками не правится. Обновляется только скиллом `change-archive` из `changes/<изменение>/specs/`.

Сценарии — источник для тестов: у каждого сценария есть тест (unit, API или e2e). Его имя пишется рядом со сценарием: `Тест: TestOrderRejectedWhenShort`.
