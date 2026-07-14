import { h } from 'vue'
import DefaultTheme from 'vitepress/theme'
import MockCanvas from './MockCanvas.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  Layout() {
    return h(DefaultTheme.Layout, null, {
      // Desktop: right of hero text (>959px)
      'home-hero-image': () => h(MockCanvas),
      // Mobile: below the entire hero section (≤959px)
      'home-hero-after': () => h('div', { class: 'sg-hero-after-wrap' }, [h(MockCanvas)]),
    })
  },
}
