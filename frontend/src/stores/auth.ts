import { defineStore } from 'pinia';
import { ref, computed } from 'vue';

export const useAuthStore = defineStore('auth', () => {
  const isAuthenticated = ref(false);
  const password = ref('');
  const role = ref<'admin' | 'guest'>('guest');

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
      if (storedRole === 'admin' || storedRole === 'guest') {
        role.value = storedRole;
      }
    }
  };

  const login = async (pwd: string) => {
    try {
      const fd = new FormData();
      fd.append('password', pwd);
      const response = await fetch('/auth', {
        method: 'POST',
        body: fd,
      });

      if (response.ok) {
        const data = await response.json();
        isAuthenticated.value = true;
        password.value = pwd;
        role.value = data.role || 'guest';

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

  return {
    isAuthenticated,
    password,
    role,
    isAdmin,
    init,
    login,
    logout,
  };
});
