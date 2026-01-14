import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useAuthStore = defineStore('auth', () => {
  const isAuthenticated = ref(false);
  const password = ref('');

  // Initialize from local storage
  const init = () => {
    const storedPassword = localStorage.getItem('server_password');
    if (storedPassword) {
      password.value = storedPassword;
      // Optimistically assume authenticated, but validation should happen on first API call
      isAuthenticated.value = true;
    }
  };

  const login = async (pwd: string) => {
    // In a real app, we would validate against the server here.
    // Based on legacy code, we just save it and use it for requests.
    // Validation happens when we try to fetch data.
    // However, the requirement says "if auth failed redirect to login".
    // So we should try to validate immediately.

    try {
      const fd = new FormData();
      fd.append('password', pwd);
      const response = await fetch('/auth', {
        method: 'POST',
        body: fd,
      });

      if (response.ok) {
        isAuthenticated.value = true;
        password.value = pwd;
        localStorage.setItem('server_password', pwd);
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
    localStorage.removeItem('server_password');
    // Router redirect should be handled by component or router
    window.location.reload();
  };

  return {
    isAuthenticated,
    password,
    init,
    login,
    logout,
  };
});
