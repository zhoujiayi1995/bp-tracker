import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '../api'
import { ApiError } from '../api/client'
import type { Me } from '../api/types'

export const useSessionStore = defineStore('session', () => {
  const me = ref<Me | null>(null)
  const loaded = ref(false)

  async function load() {
    try {
      me.value = await api.me()
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 401)) throw err
      me.value = null
    } finally {
      loaded.value = true
    }
  }

  async function login(username: string, password: string) {
    me.value = await api.login(username, password)
    loaded.value = true
  }

  async function register(username: string, password: string, displayName: string) {
    me.value = await api.register(username, password, displayName)
    loaded.value = true
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      clear()
    }
  }

  function clear() {
    me.value = null
  }

  return { me, loaded, load, login, register, logout, clear }
})
