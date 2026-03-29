import { createRouter, createRootRoute, createRoute } from '@tanstack/react-router'
import Layout from './components/Layout'
import Dashboard from './pages/Dashboard'
import Specs from './pages/Specs'
import SpecDetail from './pages/SpecDetail'
import Stats from './pages/Stats'
import Chat from './pages/Chat'

const rootRoute = createRootRoute({ component: Layout })
const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: Dashboard })
const specsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/specs', component: Specs })
const specDetailRoute = createRoute({ getParentRoute: () => rootRoute, path: '/specs/$specId', component: SpecDetail })
const statsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/stats', component: Stats })
const chatRoute = createRoute({ getParentRoute: () => rootRoute, path: '/chat', component: Chat })

const routeTree = rootRoute.addChildren([indexRoute, specsRoute, specDetailRoute, statsRoute, chatRoute])

export const router = createRouter({ routeTree, basepath: '/_ui' })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
