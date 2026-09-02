import { defineStore } from 'pinia';
import { ref, computed } from 'vue';

export type AuthRole = 'admin' | 'guest' | 'map_uploader';

const isAuthRole = (value: string | null): value is AuthRole =>
  value === 'admin' || value === 'guest' || value === 'map_uploader';

export const useAuthStore = defineStore('auth', () => {
  const isAuthenticated = ref(false);
  const password = ref('');
  const role = ref<AuthRole>('guest');

  // Initialize from local storage
  const init = () => {
    const storedPassword = localStorage.getItem('server_password');
    if (storedPassword) {
      password.value = storedPassword;
      // Optimistically assume authenticated, will be validated by API
      isAuthenticated.value = true;
      // Default to guest until validated, or maybe store role too?
      // Better to re-validate on refresh usually, but we can store role
      const storedRole = localStorage.getItem('server_role');
      if (isAuthRole(storedRole)) {
        role.value = storedRole;
      }
    }
  };

  const login = async (pwd: string) => {
    try {
      const response = await fetch('/auth', {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${pwd}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        isAuthenticated.value = true;
        password.value = pwd;
        role.value = isAuthRole(data.role) ? data.role : 'guest';

        localStorage.setItem('server_password', pwd);
        localStorage.setItem('server_role', role.value);
        return true;
      } else {
        return false;
      }
    } catch (e) {
      console.error(e);
      return false;
    }
  };

  const logout = () => {
    isAuthenticated.value = false;
    password.value = '';
    role.value = 'guest';
    localStorage.removeItem('server_password');
    localStorage.removeItem('server_role');
    window.location.reload();
  };

  const isAdmin = computed(() => role.value === 'admin');
  const isMapUploader = computed(() => role.value === 'map_uploader');
  const defaultRoute = computed(() => (isMapUploader.value ? '/map-upload' : '/'));

  return {
    isAuthenticated,
    password,
    role,
    isAdmin,
    isMapUploader,
    defaultRoute,
    init,
    login,
    logout,
  };
});
