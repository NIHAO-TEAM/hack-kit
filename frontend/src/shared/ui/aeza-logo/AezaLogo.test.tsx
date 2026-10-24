import { render, screen } from '@testing-library/react';
import { AezaLogo } from './index';

describe('AezaLogo', () => {
  it('картинка с именем «aéza», пропорции файла сохранены', () => {
    render(<AezaLogo height={34} />);
    const img = screen.getByRole('img', { name: 'aéza' });
    expect(img).toHaveAttribute('height', '34');
    expect(img).toHaveAttribute('width', String(Math.round((34 * 354) / 85)));
  });
});
