import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router';
import { bootstrapCapability } from './shared/api/client';
import { Picker, validateSearch } from './app/Picker';
import './shared/ui/styles.css';

bootstrapCapability();
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false }, mutations: { retry: false } } });
const route = createRootRoute({ component: Picker, validateSearch });
const router = createRouter({ routeTree: route, defaultPreloadStaleTime: 0 });
declare module '@tanstack/react-router' { interface Register { router: typeof router } }
createRoot(document.getElementById('root')!).render(<StrictMode><QueryClientProvider client={queryClient}><RouterProvider router={router}/></QueryClientProvider></StrictMode>);
