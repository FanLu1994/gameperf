import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import History from './views/History.vue'
import Compare from './views/Compare.vue'

const routes = [
  { path: '/', component: Dashboard },
  { path: '/history', component: History },
  { path: '/compare', component: Compare },
]

export default createRouter({
  history: createWebHistory(),
  routes,
})
