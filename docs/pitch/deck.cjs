// Презентация к защите: node docs/pitch/deck.cjs <папка скриншотов> <папка QR> <файл.pptx>.
// Требования организаторов (docs/pitch.md): pptx, без видео, гифок, анимаций и встроенных объектов,
// ссылки — только QR на прямой адрес. Числа берутся из файлов, указанных в NUM, в 10:00 25.10;
// строка «ЗАПОЛНИТЬ» в готовом файле — ошибка (проверка в конце).
const path = require('node:path');
const fs = require('node:fs');
const pptxgen = require('pptxgenjs');
const sharp = require('sharp');

const [shots, qrs, out] = process.argv.slice(2);
if (!shots || !qrs || !out) {
  console.error('нужно: <папка скриншотов> <папка QR> <файл.pptx>');
  process.exit(2);
}
const root = path.resolve(__dirname, '../..');

// Числа слайдов. Источник — в комментарии; пусто — «ЗАПОЛНИТЬ».
const NUM = {
  lossOrganizers: '3 599,07 €', // docs/model.md после funds-window (#159)
  parityMax: '300 000', // docs/load/2026-10-24-oracle-parity.md
  recalc100k: '0,65 с', // docs/load/2026-10-24-recalc-alloc.md, BenchmarkRecalc100k после
  snap100k: '5 с', // docs/load/2026-10-24-snapshot-stream.md, 100k: 4,6–5,4 с
  topShare: 'ЗАПОЛНИТЬ', // docs/model-check.md (#168): доля денег в верхних 10 %
  topShareRule: 'ЗАПОЛНИТЬ', // там же: лучшее простое правило
  eventsPerSec: 'ЗАПОЛНИТЬ', // docs/load/2026-10-24-burst.md (#156)
  burstSize: 'ЗАПОЛНИТЬ', // там же: самый большой залп без ошибок
  readP95: 'ЗАПОЛНИТЬ', // там же: p95 чтения под залпом
};

const C = {
  bg: '0A0D10', card: '151A20', line: '262E37', text: 'F3F6F8', muted: '9AA6B2',
  accent: '4CC6F1', good: '3DDC97', warn: 'F5B942',
};
const W = 13.333, H = 7.5, M = 0.6;

const pres = new pptxgen();
pres.layout = 'LAYOUT_WIDE';
pres.title = 'Контур удержания — Input Math, aéza';
pres.author = 'Input Math';
pres.theme = { headFontFace: 'Arial', bodyFontFace: 'Arial' };

const img = (f) => path.join(shots, f);
const qr = (f) => path.join(qrs, f);

function base(notes) {
  const s = pres.addSlide();
  s.background = { color: C.bg };
  if (notes) s.addNotes(notes);
  return s;
}
function title(s, t, sub) {
  s.addText(t, { x: M, y: 0.45, w: W - 2 * M, h: 0.8, fontSize: 36, bold: true, color: C.text, margin: 0, isTextBox: true });
  if (sub) s.addText(sub, { x: M, y: 1.2, w: W - 2 * M, h: 0.5, fontSize: 18, color: C.muted, margin: 0, isTextBox: true });
}
function card(s, x, y, w, h, opts = {}) {
  s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
    x, y, w, h, rectRadius: 0.12, fill: { color: opts.fill || C.card }, line: { color: C.line, width: 1 },
  });
}
function stat(s, x, y, w, value, label, color) {
  card(s, x, y, w, 1.7);
  const pending = value === 'ЗАПОЛНИТЬ';
  s.addText(value, { x: x + 0.3, y: y + 0.2, w: w - 0.6, h: 0.8, fontSize: 36, bold: true, color: pending ? C.warn : (color || C.accent), margin: 0, isTextBox: true });
  s.addText(label, { x: x + 0.3, y: y + 1.0, w: w - 0.6, h: 0.6, fontSize: 14, color: C.muted, margin: 0, valign: 'top', isTextBox: true });
}
const crops = {};
// Рабочая область экрана без бокового меню и шапки (скриншоты 1280×800 при масштабе 2), соотношение 1,6.
async function crop(file) {
  const meta = await sharp(img(file)).metadata();
  const left = 440, top = 112, width = meta.width - left, height = Math.min(Math.round(width / 1.6), meta.height - top);
  const buf = await sharp(img(file)).extract({ left, top, width, height }).png().toBuffer();
  crops[file] = 'image/png;base64,' + buf.toString('base64');
}
function shot(s, file, x, y, w) {
  const h = w / 1.6;
  s.addShape(pres.shapes.ROUNDED_RECTANGLE, { x: x - 0.04, y: y - 0.04, w: w + 0.08, h: h + 0.08, rectRadius: 0.08, fill: { color: C.line }, line: { color: C.line } });
  s.addImage({ data: crops[file], x, y, w, h, altText: 'Скриншот продукта' });
}
function bullets(s, items, x, y, w, h, size = 18) {
  s.addText(items.map((t, i) => ({ text: t, options: { bullet: { indent: 18 }, breakLine: i < items.length - 1 } })), {
    x, y, w, h, fontSize: size, color: C.text, paraSpaceAfter: 10, valign: 'top', margin: 0, isTextBox: true,
  });
}

(async () => {
  const logo = await sharp(path.join(root, 'frontend/src/shared/ui/aeza-logo/aeza-logo.svg'), { density: 600 })
    .resize({ width: 1200 }).png().toBuffer();
  const logoData = 'image/png;base64,' + logo.toString('base64');
  for (const f of ['queue.png', 'card.png', 'model.png']) await crop(f);

  // 1. Титул
  {
    const s = base('Одна фраза: внутренний сервис aéza — кого спасать сегодня, почему и сколько это стоит.');
    card(s, M, M, 2.6, 0.95, { fill: 'FFFFFF' });
    s.addImage({ data: logoData, x: M + 0.25, y: M + 0.22, w: 2.1, h: 2.1 * 85 / 354, altText: 'Логотип aéza' });
    s.addText('Контур удержания', { x: M, y: 2.3, w: 8, h: 1.1, fontSize: 54, bold: true, color: C.text, margin: 0, isTextBox: true });
    s.addText('Кого из клиентов aéza спасать сегодня, почему и сколько это стоит', { x: M, y: 3.4, w: 7.6, h: 1.0, fontSize: 22, color: C.muted, margin: 0, isTextBox: true });
    s.addText('Команда Input Math · кейс aéza «Анализатор рисков оттока»', { x: M, y: 6.3, w: 8, h: 0.5, fontSize: 16, color: C.accent, margin: 0, isTextBox: true });
    card(s, 9.4, 2.0, 3.3, 3.9, { fill: 'FFFFFF' });
    s.addImage({ path: qr('qr-prod.png'), x: 9.6, y: 2.2, w: 2.9, h: 2.9, altText: 'QR: прод' });
    s.addText('Открыть продукт', { x: 9.4, y: 5.2, w: 3.3, h: 0.5, fontSize: 16, bold: true, color: '0A0D10', align: 'center', margin: 0, isTextBox: true });
  }

  // 2. Задача
  {
    const s = base('Кейс: три вопроса. Истории оттока нет — только снимок и поток изменений.');
    title(s, 'Маркетологу нужны три ответа', 'Истории оттока в данных нет: только текущий снимок биллинга и поток изменений');
    const q = [
      ['Кому?', 'Клиенты, которые могут не продлить услуги, — в порядке денег под риском'],
      ['Почему?', 'Причина простыми словами: автопродление выключено, денег не хватает, ждёт оплаты'],
      ['Сколько?', 'Ожидаемая потеря в евро за 30 дней и действие, которое её снимает'],
    ];
    q.forEach(([h, t], i) => {
      const x = M + i * 4.1;
      card(s, x, 2.3, 3.8, 3.6);
      s.addText(h, { x: x + 0.35, y: 2.6, w: 3.1, h: 0.9, fontSize: 40, bold: true, color: C.accent, margin: 0, isTextBox: true });
      s.addText(t, { x: x + 0.35, y: 3.6, w: 3.1, h: 2.1, fontSize: 18, color: C.text, valign: 'top', margin: 0, isTextBox: true });
    });
  }

  // 3. Очередь
  {
    const s = base('Очередь по ожидаемой потере, а не по скору. Три числа сверху — что значит каждое. Действие с ценой в каждой строке.');
    title(s, 'Ответ на одном экране');
    shot(s, 'queue.png', M, 1.45, 8.4);
    const tx = M + 8.75, tw = W - tx - M;
    bullets(s, [
      'Очередь по деньгам: наверху тот, на ком больше всего теряем',
      'Причина — словами, без кодов',
      'У каждого — одно действие и сколько евро оно снимает',
      'Сбой автосписания — «к биллингу», не маркетологу',
    ], tx, 1.6, tw, 4.8, 17);
  }

  // 4. Карточка
  {
    const s = base('Клиент 9: пополнить баланс — снимает больше всего. Это пересчёт «клиент выполнил действие полностью», не обещание окупаемости.');
    title(s, 'Почему этот клиент и что с ним делать');
    shot(s, 'card.png', M, 1.45, 8.4);
    const tx = M + 8.75, tw = W - tx - M;
    bullets(s, [
      'Сравнение действий: цена, сколько снимает, «на каждый €»',
      'Сумма по услугам равна потере клиента до цента',
      'Признаки с весом и вкладом — формулу видно',
      'Честно: отклик клиента и цена контакта не учтены',
    ], tx, 1.6, tw, 4.8, 17);
  }

  // 5. Модель
  {
    const s = base('Шумовое ИЛИ по признакам услуги, худшая услуга — скор клиента, P × деньги услуги за 30 дней. Веса меняются на экране «Модель», очередь перестраивается без перезагрузки.');
    title(s, 'Модель, которую можно проверить руками');
    const steps = ['Признаки услуги', 'Риск услуги P', 'Скор клиента', 'Потеря в € за 30 дней'];
    steps.forEach((t, i) => {
      const x = M + i * 3.1;
      card(s, x, 1.7, 2.7, 1.1, { fill: i === 3 ? '0E3A4A' : C.card });
      s.addText(t, { x: x + 0.15, y: 1.7, w: 2.4, h: 1.1, fontSize: 18, bold: true, color: C.text, align: 'center', valign: 'middle', margin: 0, isTextBox: true });
      if (i < 3) s.addShape(pres.shapes.RIGHT_TRIANGLE, { x: x + 2.78, y: 2.1, w: 0.25, h: 0.3, rotate: 90, fill: { color: C.accent }, line: { color: C.accent } });
    });
    s.addText('P = 1 − Π(1 − wᵢ·fᵢ)   ·   скор = 100 · max P   ·   потеря = Σ round(P · R)', { x: M, y: 3.1, w: W - 2 * M, h: 0.6, fontSize: 16, color: C.accent, fontFace: 'Courier New', margin: 0, isTextBox: true });
    stat(s, M, 4.2, 3.8, NUM.parityMax, 'клиентов — расчёт Go совпал с независимым эталоном на Python до цента', C.good);
    stat(s, M + 4.1, 4.2, 3.8, NUM.lossOrganizers, 'ожидаемая потеря на данных организаторов — воспроизводится вручную', C.accent);
    stat(s, M + 8.2, 4.2, 3.9, '1 файл', 'новый признак: файл, правило, эталон, тест', C.accent);
  }

  // 6. Проверка модели
  {
    const s = base('Истории нет, поэтому проверяем на прокрутке 30 дней с названным поведением клиента. Это симуляция по нашим допущениям, не данные aéza и не калибровка вероятности.');
    title(s, 'Модель ставит наверх тех, кто уходит', 'Прокрутка 30 дней: правила биллинга + поведение клиента со случайностью (docs/model-check.md)');
    stat(s, M, 2.2, 5.8, NUM.topShare, 'потерянных денег в верхних 10 % нашей очереди', C.good);
    stat(s, M + 6.3, 2.2, 5.8, NUM.topShareRule, 'у лучшего простого правила («только ожидание оплаты» и др.)', C.muted);
    bullets(s, [
      'Вероятности поведения названы и не взяты из весов модели',
      'Устойчивость: сдвиг каждой вероятности ±50 %',
      'Где модель проигрывает правилу — показываем, не прячем',
    ], M, 4.4, W - 2 * M, 2.4, 18);
  }

  // 7. Ограничения
  {
    const s = base('Говорим сами, до вопроса: веса экспертные, признаки зависимы, доли по причинам — не причинность.');
    title(s, 'Что мы знаем о слабых местах модели');
    const items = [
      ['Веса — экспертная оценка', 'Истории исходов нет; веса редактируются аналитиком, не подобраны по данным'],
      ['Признаки зависимы', 'Нет денег → сбой продления → ожидание оплаты: одна причина учитывается несколько раз'],
      ['Доли потерь — по вкладу', 'Показывают, где сосредоточен риск, но не доказывают причинность'],
    ];
    items.forEach(([h, t], i) => {
      const y = 1.7 + i * 1.75;
      card(s, M, y, W - 2 * M, 1.5);
      s.addText(h, { x: M + 0.35, y: y + 0.2, w: 4.2, h: 1.1, fontSize: 22, bold: true, color: C.warn, valign: 'middle', margin: 0, isTextBox: true });
      s.addText(t, { x: M + 4.8, y: y + 0.2, w: W - 2 * M - 5.1, h: 1.1, fontSize: 18, color: C.text, valign: 'middle', margin: 0, isTextBox: true });
    });
  }

  // 8. Архитектура
  {
    const s = base('Один процесс Go: приём, шина в памяти, пересчёт на всех ядрах с отменой, неизменяемая модель чтения. Источник правды — PostgreSQL. Почему без Kafka, Redis и ClickHouse — ADR-0009.');
    title(s, 'Ядро — один процесс Go на всех ядрах', 'Без сети между приёмом, расчётом и выдачей. Источник правды — PostgreSQL');
    const box = (t, x, y, w, fill) => {
      card(s, x, y, w, 0.95, { fill });
      s.addText(t, { x, y, w, h: 0.95, fontSize: 16, bold: true, color: C.text, align: 'center', valign: 'middle', margin: 0, isTextBox: true });
    };
    const arrow = (x1, y1, x2, y2) => s.addShape(pres.shapes.LINE, { x: x1, y: y1, w: x2 - x1, h: y2 - y1, line: { color: C.accent, width: 2, endArrowType: 'triangle' } });
    box('Биллинг / эмулятор', M, 2.4, 2.4);
    card(s, 3.4, 1.95, 6.6, 4.0, { fill: '0F1419' });
    s.addText('процесс Go', { x: 3.6, y: 2.0, w: 3, h: 0.4, fontSize: 13, color: C.muted, margin: 0, isTextBox: true });
    box('Приём пачками', 3.7, 2.4, 2.8);
    box('Шина в памяти', 6.9, 2.4, 2.8);
    box('Пересчёт · N ядер', 6.9, 3.75, 2.8);
    box('Модель чтения', 3.7, 3.75, 2.8);
    box('REST · SSE · /metrics', 3.7, 5.0, 6.0);
    box('PostgreSQL', 10.4, 2.4, 2.3, '0E3A4A');
    box('Браузер · Prometheus', 10.4, 5.0, 2.3);
    arrow(M + 2.4, 2.87, 3.7, 2.87);
    arrow(6.5, 2.87, 6.9, 2.87);
    arrow(9.7, 2.87, 10.4, 2.87);
    arrow(8.3, 3.35, 8.3, 3.75);
    arrow(6.9, 4.22, 6.5, 4.22);
    arrow(5.1, 4.7, 5.1, 5.0);
    arrow(9.7, 5.47, 10.4, 5.47);
    s.addText('Kafka, Redis, ClickHouse — не нужны на этих объёмах; когда нужны — ADR-0009', { x: M, y: 6.4, w: W - 2 * M, h: 0.5, fontSize: 15, color: C.muted, margin: 0, isTextBox: true });
  }

  // 8б. Путь одного изменения
  {
    const s = base('Путь изменения из README «Как это устроено внутри»: у каждого шага файл и тест. Здесь Go работает по максимуму: каналы с обратным давлением, воркеры по ядрам с отменой, атомарная публикация результата.');
    title(s, 'Путь изменения: от биллинга до экрана');
    const steps = [
      ['Приём', 'снимок потоком, каналы с обратным давлением — память не растёт'],
      ['Запись', 'COPY в одной транзакции, ошибка — откат всего'],
      ['Сигнал', 'шина в памяти, медленный подписчик не тормозит'],
      ['Пересчёт', 'воркеры по ядрам, отмена через context'],
      ['Модель', 'признаки → риск → деньги в центах, сверка с эталоном'],
      ['Выдача', 'атомарная публикация, REST и SSE без блокировок'],
    ];
    steps.forEach(([h, t], i) => {
      const col = i % 3, row = Math.floor(i / 3);
      const x = M + col * 4.1, y = 1.6 + row * 2.6;
      card(s, x, y, 3.8, 2.3);
      s.addText(String(i + 1), { x: x + 0.3, y: y + 0.2, w: 0.6, h: 0.7, fontSize: 32, bold: true, color: C.accent, margin: 0, isTextBox: true });
      s.addText(h, { x: x + 0.95, y: y + 0.25, w: 2.6, h: 0.6, fontSize: 22, bold: true, color: C.text, margin: 0, valign: 'middle', isTextBox: true });
      s.addText(t, { x: x + 0.3, y: y + 1.0, w: 3.2, h: 1.15, fontSize: 16, color: C.muted, valign: 'top', margin: 0, isTextBox: true });
    });
  }

  // 9. Нагрузка
  {
    const s = base('Числа только из docs/load с железом: 4 vCPU. Недостигнутые цели называем.');
    title(s, 'Гонится и не падает', 'Портфель 100 000 клиентов, параллельное чтение и живая лента · 4 vCPU (docs/load)');
    stat(s, M, 2.0, 3.9, NUM.snap100k, 'загрузка снимка 100 000 клиентов, 1,4 млн строк (было 42–45 с)');
    stat(s, M + 4.1, 2.0, 3.9, NUM.recalc100k, 'расчёт 100 000 клиентов на всех ядрах');
    stat(s, M + 8.2, 2.0, 3.9, '0', 'расхождений с эталоном на ' + NUM.parityMax + ' клиентов', C.good);
    stat(s, M, 4.1, 3.9, NUM.eventsPerSec, 'событий в секунду на 100 000 клиентов');
    stat(s, M + 4.1, 4.1, 3.9, NUM.burstSize, 'событий в залпе: 0 ошибок, 0 потерянных, итог совпал до цента', C.good);
    stat(s, M + 8.2, 4.1, 3.9, NUM.readP95, 'p95 чтения очереди во время залпа');
  }

  // 10. Встраивание
  {
    const s = base('Helm-чарт: адрес биллинга, OIDC, Prometheus — правкой values.yaml. /metrics — те же числа, что в панели.');
    title(s, 'Встаёт в контур aéza правкой values.yaml');
    shot(s, 'model.png', M, 1.45, 7.2);
    const tx = M + 7.6, tw = W - tx - M;
    bullets(s, [
      'Kubernetes и Helm: выкат без простоя',
      'Вход через Keycloak с TOTP, пароль — запасной',
      '/metrics и Grafana — те же числа, что в панели',
      'Поток событий биллинга — пачкой по HTTP или адаптером к вашей шине',
      'Права ролей проверяются на сервере',
    ], tx, 1.6, tw, 5.2, 17);
  }

  // 11. Итог
  {
    const s = base('Полезное, объяснимое, расширяемое и работающее — по словам критерия жюри.');
    title(s, 'Итог');
    const four = [
      ['Полезное', 'очередь по деньгам и действие с ценой'],
      ['Объяснимое', 'каждое число воспроизводит эталон'],
      ['Расширяемое', 'признак — файл, контур — values.yaml'],
      ['Работающее', 'поток и залп под нагрузкой без ошибок'],
    ];
    four.forEach(([h, t], i) => {
      const x = M + (i % 2) * 3.9, y = 1.6 + Math.floor(i / 2) * 2.35;
      card(s, x, y, 3.65, 2.1);
      s.addText(h, { x: x + 0.3, y: y + 0.25, w: 3.1, h: 0.6, fontSize: 24, bold: true, color: C.accent, margin: 0, isTextBox: true });
      s.addText(t, { x: x + 0.3, y: y + 0.95, w: 3.1, h: 1.0, fontSize: 17, color: C.text, valign: 'top', margin: 0, isTextBox: true });
    });
    [['qr-prod.png', 'Продукт'], ['qr-repo.png', 'Репозиторий'], ['qr-swagger.png', 'Swagger']].forEach(([f, t], i) => {
      const x = 8.75 + (i % 2) * 2.1, y = 1.6 + Math.floor(i / 2) * 2.55;
      card(s, x, y, 1.9, 2.35, { fill: 'FFFFFF' });
      s.addImage({ path: qr(f), x: x + 0.15, y: y + 0.15, w: 1.6, h: 1.6, altText: 'QR: ' + t });
      s.addText(t, { x, y: y + 1.8, w: 1.9, h: 0.45, fontSize: 14, bold: true, color: '0A0D10', align: 'center', margin: 0, isTextBox: true });
    });
  }

  await pres.writeFile({ fileName: out });
  const left = Object.entries(NUM).filter(([, v]) => v === 'ЗАПОЛНИТЬ').map(([k]) => k);
  console.log(out);
  if (left.length) console.log('НЕ ЗАПОЛНЕНО: ' + left.join(', '));
})();
