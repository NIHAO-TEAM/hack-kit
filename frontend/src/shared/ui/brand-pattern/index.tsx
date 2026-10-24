/**
 * Фирменный паттерн (ADR-0003): три параллельные линии и одна акцентная, поворачивающая со скруглением.
 * Декор: только экран входа и пустые состояния, не рабочие экраны.
 */
export function BrandPattern({ width = 320, height = 120 }: { width?: number; height?: number }) {
  return (
    <svg
      aria-hidden="true"
      focusable="false"
      width={width}
      height={height}
      viewBox="0 0 320 120"
      fill="none"
      strokeWidth="3"
      strokeLinecap="round"
      style={{ display: 'block', maxWidth: '100%', pointerEvents: 'none' }}
    >
      <g stroke="var(--border-strong)">
        <path d="M0 20H320" />
        <path d="M0 40H320" />
        <path d="M0 60H320" />
      </g>
      <path d="M0 80H220a24 24 0 0 1 24 24" stroke="var(--primary)" />
    </svg>
  );
}
