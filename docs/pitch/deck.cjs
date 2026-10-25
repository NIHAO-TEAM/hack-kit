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
  topShare: '71,8 %', // docs/model-check.md, 100 000 аккаунтов: модель, деньги в верхних 10 %
  topShareNoMoney: '6–22 %', // там же: правила без денег (prolong_wait, no_autoprolong, low_funds)
  topSharePrice: '72,5 %', // там же: правило «по цене услуги»
  rocModel: '0,660', rocBestRule: '0,652', // там же: ROC AUC модели и лучшего правила
  eventsPerSec: '20 700', // docs/load/2026-10-24-ingest-delta.md: 100 000 клиентов, залп 50 000, synchronous_commit off
  burstSize: '50 000', // там же: применено 50 000 из 50 000, согласованность 0 ¢
  podKillReads: '29 730', // Deploy chaos 40155919862 (main 701add3): удаление пода под залпом 10 000, 0 ошибок чтения, 10 000 из 10 000, 0 ¢; выкат — 35 754 чтения, 0 ошибок
  readP95: '49 мс', // Deploy burst 40137862240: p95 чтения очереди во время залпа 50 000 (20 читателей, 500 SSE), 0 × 5xx, память 1,9 ГБ
  snap1m: '143–183 с', // docs/load/2026-10-24-scale-1m.md: PUT /source/snapshot потоком, 1 000 000 клиентов, ≈ 1,07 ГиБ JSON
  recalc1m: '21–25 с', // там же: полный пересчёт 1 000 000 клиентов (churn_recalc_duration_seconds)
  read1m: '162 мс', // там же: p95 чтения очереди, k6 read.js, 0 % ошибок (прогон 40139932904)
  mem1m: '11 ГБ', // там же: пик RSS приложения 10,7–11,1 ГБ из 16
};

// Светлая минималистичная тема: одна мысль на слайд, крупный текст, 7 слайдов на 3 минуты.
const C = {
  bg: 'FFFFFF', line: 'DDE3EA', text: '0B0F14', muted: '5B6573',
  accent: '0A7FBF', good: '138A5B', warn: 'B7791F',
};
const W = 13.333, M = 0.8;

const pres = new pptxgen();
pres.layout = 'LAYOUT_WIDE';
pres.title = 'Контур удержания — Input Math, aéza';
pres.author = 'Input Math';
pres.theme = { headFontFace: 'Arial', bodyFontFace: 'Arial' };

const img = (f) => path.join(shots, f);
const qr = (f) => path.join(qrs, f);
// Пробел внутри числа неразрывный: иначе число рвётся на две строки.
const nb = (t) => t.replace(/(\d) (?=\d{3}(\D|$))/g, '$1 ');

function base(notes) {
  const s = pres.addSlide();
  s.background = { color: C.bg };
  if (notes) s.addNotes(notes);
  return s;
}
function title(s, t) {
  s.addText(t, { x: M, y: 0.55, w: W - 2 * M, h: 0.9, fontSize: 40, bold: true, color: C.text, margin: 0, isTextBox: true });
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
  s.addShape(pres.shapes.RECTANGLE, { x: x - 0.03, y: y - 0.03, w: w + 0.06, h: h + 0.06, fill: { color: C.line }, line: { color: C.line } });
  s.addImage({ data: crops[file], x, y, w, h, altText: 'Скриншот продукта' });
}
// Колонка коротких тезисов справа от скриншота: крупно, без маркеров.
function points(s, items, x, y, w) {
  items.forEach(([h, t], i) => {
    const yy = y + i * 1.45;
    s.addText(nb(h), { x, y: yy, w, h: 0.5, fontSize: 24, bold: true, color: C.accent, margin: 0, isTextBox: true });
    s.addText(nb(t), { x, y: yy + 0.5, w, h: 0.8, fontSize: 16, color: C.muted, valign: 'top', margin: 0, isTextBox: true });
  });
}
// Большое число и подпись под ним.
function big(s, x, y, w, value, label, color) {
  s.addText(nb(value), { x, y, w, h: 1.0, fontSize: 50, bold: true, color: color || C.accent, margin: 0, isTextBox: true });
  s.addText(nb(label), { x, y: y + 1.05, w, h: 0.9, fontSize: 17, color: C.muted, valign: 'top', margin: 0, isTextBox: true });
}

(async () => {
  const logo = await sharp(path.join(root, 'frontend/src/shared/ui/aeza-logo/aeza-logo.svg'), { density: 600 })
    .resize({ width: 1200 }).png().toBuffer();
  const logoData = 'image/png;base64,' + logo.toString('base64');
  for (const f of ['queue.png', 'card.png']) await crop(f);

  // 1. Титул
  {
    const s = base('Команда Input Math, кейс aéza. Внутренний сервис: кого из клиентов спасать сегодня, почему и сколько это стоит.');
    s.addImage({ data: logoData, x: M, y: 0.7, w: 2.0, h: 2.0 * 85 / 354, altText: 'Логотип aéza' });
    s.addText('Контур удержания', { x: M, y: 2.4, w: 8, h: 1.2, fontSize: 60, bold: true, color: C.text, margin: 0, isTextBox: true });
    s.addText('Кого из клиентов спасать сегодня, почему и сколько это стоит', { x: M, y: 3.7, w: 7.6, h: 1.0, fontSize: 24, color: C.muted, margin: 0, isTextBox: true });
    s.addText('Input Math · кейс «Анализатор рисков оттока»', { x: M, y: 6.3, w: 8, h: 0.5, fontSize: 18, color: C.accent, margin: 0, isTextBox: true });
    s.addImage({ path: qr('qr-prod.png'), x: 9.6, y: 2.3, w: 2.9, h: 2.9, altText: 'QR: прод' });
    s.addText('Открыть продукт', { x: 9.4, y: 5.3, w: 3.3, h: 0.5, fontSize: 16, bold: true, color: C.text, align: 'center', margin: 0, isTextBox: true });
  }

  // 2. Задача
  {
    const s = base('Три вопроса маркетолога. Истории оттока нет — только снимок биллинга и поток изменений, поэтому модель объяснимая, а не обученная.');
    title(s, 'Маркетологу нужны три ответа');
    [['Кому?', 'кто может не продлить услуги'], ['Почему?', 'причина простыми словами'], ['Сколько?', 'потеря в евро и действие']].forEach(([h, t], i) => {
      const x = M + i * 4.0;
      s.addText(h, { x, y: 2.5, w: 3.7, h: 1.2, fontSize: 54, bold: true, color: C.accent, margin: 0, isTextBox: true });
      s.addText(t, { x, y: 3.8, w: 3.5, h: 1.0, fontSize: 22, color: C.text, valign: 'top', margin: 0, isTextBox: true });
    });
    s.addText('Истории оттока в данных нет — только снимок биллинга и поток изменений', { x: M, y: 6.2, w: W - 2 * M, h: 0.5, fontSize: 18, color: C.muted, margin: 0, isTextBox: true });
  }

  // 3. Очередь
  {
    const s = base('Очередь по деньгам: наверху тот, на ком теряем больше всего. У каждого причина словами и одно действие с ценой.');
    title(s, 'Очередь по деньгам, а не по скору');
    shot(s, 'queue.png', M, 1.75, 8.0);
    points(s, [
      ['Кому', 'наверху — на ком теряем больше всего'],
      ['Почему', 'причина словами, без кодов'],
      ['Что делать', 'одно действие и сколько € оно снимает'],
    ], M + 8.4, 1.9, W - M - (M + 8.4));
  }

  // 4. Карточка
  {
    const s = base('Клиент 9: ожидаемая потеря 3 189,10 €. Пополнить баланс на 529,84 € снимает 1 027,49 €. Причины — по услугам. Отклик клиента и цена контакта не учтены.');
    title(s, 'Клиент: причина и действие с ценой');
    shot(s, 'card.png', M, 1.75, 8.0);
    points(s, [
      ['3 189 €', 'ожидаемая потеря клиента за 30 дней'],
      ['−1 027 €', 'снимает действие «пополнить баланс»'],
      ['По услугам', 'сколько теряем на каждой и что не так'],
    ], M + 8.4, 1.9, W - M - (M + 8.4));
  }

  // 5. Модель
  {
    const s = base('Модель объяснимая: риск услуги × её деньги за 30 дней. Go совпал с эталоном на Python до цента на 300 000 клиентов. Прокрутка 30 дней на 100 000: в верхних 10 % очереди 71,8 % потерянных денег; правило «по цене» — 72,5 %, правила без денег — 6–22 %. Симуляция, не данные aéza. Веса экспертные.');
    title(s, 'Каждое число можно проверить');
    big(s, M, 2.3, 3.7, NUM.parityMax, 'клиентов — расчёт совпал с независимым эталоном до цента', C.good);
    big(s, M + 4.0, 2.3, 3.7, NUM.topShare, 'потерянных денег — в верхних 10 % очереди (прокрутка 30 дней)');
    big(s, M + 8.0, 2.3, 3.7, '1 файл', 'новый признак: правило, эталон, тест');
    s.addText('Истории нет — веса экспертные, меняются на экране «Модель». Это симуляция, не данные aéza.', { x: M, y: 6.1, w: W - 2 * M, h: 0.6, fontSize: 17, color: C.muted, margin: 0, isTextBox: true });
  }

  // 6. Нагрузка и большие данные
  {
    const s = base('100 000 клиентов: пересчёт 0,65 с, залп 50 000 событий без потерь, p95 чтения 49 мс. 1 000 000: пересчёт 21–25 с, чтение 162 мс, память 11 ГБ из 16. Узкое место — залп событий на миллионе: пересчёт не догоняет поток; дальше — хранить портфель частями (ADR-0009).');
    title(s, 'Большие данные: где упираемся');
    s.addText('100 000 клиентов', { x: M, y: 1.65, w: 6, h: 0.5, fontSize: 20, bold: true, color: C.text, margin: 0, isTextBox: true });
    big(s, M, 2.2, 3.7, NUM.recalc100k, 'полный пересчёт');
    big(s, M + 4.0, 2.2, 3.7, NUM.burstSize, 'событий в залпе — 0 потерь, итог до цента', C.good);
    big(s, M + 8.0, 2.2, 3.7, NUM.readP95, 'p95 чтения очереди во время залпа');
    s.addShape(pres.shapes.LINE, { x: M, y: 4.45, w: W - 2 * M, h: 0, line: { color: C.line, width: 1 } });
    s.addText('1 000 000 клиентов', { x: M, y: 4.7, w: 6, h: 0.5, fontSize: 20, bold: true, color: C.text, margin: 0, isTextBox: true });
    s.addText([
      { text: nb(NUM.recalc1m), options: { bold: true, color: C.accent } }, { text: ' пересчёт   ·   ', options: { color: C.muted } },
      { text: nb(NUM.read1m), options: { bold: true, color: C.accent } }, { text: ' чтение   ·   ', options: { color: C.muted } },
      { text: nb(NUM.mem1m), options: { bold: true, color: C.warn } }, { text: ' памяти из 16', options: { color: C.muted } },
    ], { x: M, y: 5.25, w: W - 2 * M, h: 0.6, fontSize: 24, margin: 0, isTextBox: true });
    s.addText('Узкое место: залп событий на 1M — пересчёт не догоняет поток. Дальше — хранить портфель частями.', { x: M, y: 6.1, w: W - 2 * M, h: 0.6, fontSize: 18, bold: true, color: C.warn, margin: 0, isTextBox: true });
  }

  // 7. Стек и итог
  {
    const s = base('Стек: Go и PostgreSQL, React и TypeScript, Kubernetes и Helm, Keycloak, Prometheus и Grafana. Встаёт в контур aéza правкой values.yaml. QR — продукт, репозиторий, Swagger.');
    title(s, 'Стек и как встроить');
    const stack = [
      ['Backend', 'Go · PostgreSQL'],
      ['Frontend', 'React · TypeScript · Mantine'],
      ['Инфраструктура', 'Kubernetes · Helm · Keycloak'],
      ['Метрики', 'Prometheus · Grafana'],
    ];
    stack.forEach(([h, t], i) => {
      const y = 1.9 + i * 1.0;
      s.addText(h, { x: M, y, w: 2.8, h: 0.6, fontSize: 20, bold: true, color: C.accent, margin: 0, valign: 'middle', isTextBox: true });
      s.addText(t, { x: M + 2.9, y, w: 4.6, h: 0.6, fontSize: 20, color: C.text, margin: 0, valign: 'middle', isTextBox: true });
    });
    s.addText('В контур aéza — правкой одного values.yaml', { x: M, y: 6.0, w: 7.4, h: 0.6, fontSize: 20, bold: true, color: C.text, margin: 0, isTextBox: true });
    [['qr-prod.png', 'Продукт'], ['qr-repo.png', 'Репозиторий'], ['qr-swagger.png', 'Swagger']].forEach(([f, t], i) => {
      const x = 8.3 + (i % 2) * 2.2, y = 1.8 + Math.floor(i / 2) * 2.55;
      s.addImage({ path: qr(f), x, y, w: 1.9, h: 1.9, altText: 'QR: ' + t });
      s.addText(t, { x, y: y + 1.95, w: 1.9, h: 0.4, fontSize: 14, bold: true, color: C.text, align: 'center', margin: 0, isTextBox: true });
    });
  }

  await pres.writeFile({ fileName: out });
  const left = Object.entries(NUM).filter(([, v]) => v === 'ЗАПОЛНИТЬ').map(([k]) => k);
  console.log(out);
  if (left.length) console.log('НЕ ЗАПОЛНЕНО: ' + left.join(', '));
})();
