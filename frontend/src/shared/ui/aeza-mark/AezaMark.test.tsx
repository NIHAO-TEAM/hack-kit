import { render } from '@testing-library/react';
import { AezaMark } from './index';

describe('AezaMark', () => {
  it('декор: скрыт от читалок, квадрат заданного размера', () => {
    const { container } = render(<AezaMark size={56} />);
    const img = container.querySelector('img')!;
    expect(img).toHaveAttribute('aria-hidden', 'true');
    expect(img).toHaveAttribute('alt', '');
    expect(img).toHaveAttribute('width', '56');
    expect(img).toHaveAttribute('height', '56');
  });
});
