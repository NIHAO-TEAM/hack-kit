import { createFileRoute } from '@tanstack/react-router';
import { DevUiPage } from '@pages/dev-ui';

export const Route = createFileRoute('/_app/dev/ui')({ component: DevUiPage });
