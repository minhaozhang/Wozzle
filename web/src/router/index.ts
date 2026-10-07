import { createRouter, createWebHistory } from 'vue-router'
import DashboardPage from '../pages/DashboardPage.vue'
import LogsPage from '../pages/LogsPage.vue'
import InspectPage from '../pages/InspectPage.vue'
import TerminalPage from '../pages/TerminalPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardPage },
    { path: '/container/:id/logs', name: 'logs', component: LogsPage },
    { path: '/container/:id/inspect', name: 'inspect', component: InspectPage },
    { path: '/container/:id/terminal', name: 'terminal', component: TerminalPage },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
