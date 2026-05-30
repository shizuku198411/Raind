<template>
  <div class="login-page">
    <section class="login-card">
      <img src="/raind_icon.png" alt="Raind icon" class="login-icon" />
      <h1>Raind WebUI</h1>
      <p class="sub">Sign in to continue.</p>
      <label class="field">
        <span>Username</span>
        <input v-model.trim="loginForm.username" type="text" autocomplete="username" />
      </label>
      <label class="field">
        <span>Password</span>
        <input
          v-model="loginForm.password"
          type="password"
          autocomplete="current-password"
          @keyup.enter="onSubmitLogin"
        />
      </label>
      <p v-if="authError" class="error">{{ authError }}</p>
      <button class="primary" :disabled="authSubmitting || !loginReady" @click="onSubmitLogin">
        {{ authSubmitting ? 'Signing in...' : 'Login' }}
      </button>
    </section>
  </div>
</template>

<script setup>
defineProps({
  authSubmitting: { type: Boolean, required: true },
  loginForm: { type: Object, required: true },
  loginReady: { type: Boolean, required: true },
  authError: { type: String, default: '' },
  onSubmitLogin: { type: Function, required: true }
})
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: radial-gradient(circle at top, #252a35 0%, #1e2025 42%);
}

.login-card {
  width: min(420px, 92vw);
  border: 1px solid #3a4150;
  border-radius: 14px;
  background: #1f232b;
  padding: 20px;
  display: grid;
  gap: 10px;
  box-shadow: 0 18px 46px rgba(0, 0, 0, 0.35);
}

.login-icon {
  width: 60px;
  height: 60px;
  object-fit: contain;
}

h1 {
  margin: 0;
  font-size: 22px;
  color: #f2f5fb;
}

.sub {
  margin: 0 0 6px;
  color: #adb6c9;
}

.field {
  display: grid;
  gap: 6px;
}

.field span {
  color: #adb6c9;
  font-size: 12px;
}

.field input {
  border: 1px solid #3a4150;
  background: #232a35;
  color: #f2f5fb;
  border-radius: 8px;
  padding: 8px 10px;
}

.field input:focus {
  outline: none;
  border-color: #1789ff;
}

.error {
  margin: 0;
  color: #ff7b7b;
  font-size: 13px;
}

.primary {
  margin-top: 4px;
  border: 1px solid #1789ff;
  background: #1789ff;
  color: #f2f5fb;
  border-radius: 8px;
  padding: 9px 12px;
  font-weight: 600;
  cursor: pointer;
}

.primary:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}
</style>
