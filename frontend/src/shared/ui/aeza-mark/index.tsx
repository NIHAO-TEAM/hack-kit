import markUrl from './aeza-mark.svg';

/**
 * Официальный значок aéza (ADR-0003): файл без правок, только квадрат.
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
