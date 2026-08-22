import { createApp } from 'vue'
import { createPinia } from 'pinia'
import TinyVue from '@opentiny/vue'
import '@opentiny/vue-theme/index.css'
import './styles/tokens.css'
import App from './App.vue'
import router from './router'
import { permission } from './directives/permission'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(TinyVue)
app.directive('permission', permission)

app.mount('#app')
