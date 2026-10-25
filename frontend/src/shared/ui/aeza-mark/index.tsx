import markUrl from './aeza-mark.svg';

/**
 * Значок aéza: первая клавиша «a» официального логотипа (тот же файл, viewBox обрезан до квадрата).
 * Прежний значок с my.aeza.net — буква «Л» личного кабинета, для продукта не подходит.
 * Охранное поле вокруг задаёт место, где значок стоит.
 */
export function AezaMark({ size }: { size: number }) {
  return (
    <img
      src={markUrl}
      alt=""
      aria-hidden="true"
      width={size}
      height={size}
      style={{ display: 'block', flexShrink: 0 }}
    />
  );
}
