import { createRouter, createWebHashHistory } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import MainLayout from '../layouts/MainLayout.vue';
import Login from '../views/Login.vue';
import Home from '../views/Home.vue';
// Lazy load other views
const Maps = () => import('../views/Maps.vue');
const Rcon = () => import('../views/Rcon.vue');
const System = () => import('../views/System.vue');

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: Login,
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      component: MainLayout,
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'Home',
          component: Home,
        },
        {
          path: 'maps',
          name: 'Maps',
          component: Maps,
        },
        {
          path: 'rcon',
          name: 'Rcon',
          component: Rcon,
        },
        {
          path: 'system',
          name: 'System',
          component: System,
        },
      ],
    },
  ],
});

router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore();

  // Try to init auth from local storage if not already done
  if (!authStore.isAuthenticated && localStorage.getItem('server_password')) {
    authStore.init();
  }

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login');
  } else if (to.path === '/login' && authStore.isAuthenticated) {
    next('/');
  } else {
    next();
  }
});

export default router;
