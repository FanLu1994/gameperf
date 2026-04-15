<template>
  <div class="app">
    <el-container>
      <el-header class="header">
        <div class="logo" @click="$router.push('/')">
          <span class="icon">🎮</span>
          <span class="title">GamePerf</span>
          <span class="subtitle">游戏性能监控面板</span>
        </div>
        <el-menu mode="horizontal" :default-active="$route.path" router>
          <el-menu-item index="/">实时监控</el-menu-item>
          <el-menu-item index="/history">历史记录</el-menu-item>
          <el-menu-item index="/compare">对比分析</el-menu-item>
        </el-menu>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </div>
</template>

<script setup>
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0a0a1a; color: #e0e0e0; }
.app { min-height: 100vh; }
.header {
  display: flex; align-items: center; justify-content: space-between;
  background: #111128; border-bottom: 1px solid #2a2a4a; padding: 0 24px; height: 60px;
}
.logo { display: flex; align-items: center; gap: 10px; cursor: pointer; }
.logo .icon { font-size: 24px; }
.logo .title { font-size: 20px; font-weight: 700; color: #00d4ff; }
.logo .subtitle { font-size: 12px; color: #666; margin-left: 4px; }
.el-menu--horizontal { background: transparent !important; border: none !important; }
.el-menu--horizontal .el-menu-item { color: #aaa !important; border-bottom: 2px solid transparent !important; }
.el-menu--horizontal .el-menu-item.is-active { color: #00d4ff !important; border-bottom-color: #00d4ff !important; }
.main { padding: 20px; max-width: 1400px; margin: 0 auto; }

/* 全局暗色覆盖 */
.el-card { background: #151530 !important; border-color: #2a2a4a !important; color: #e0e0e0 !important; }
.el-card__header { border-bottom-color: #2a2a4a !important; color: #00d4ff !important; }
.el-table { background: transparent !important; --el-table-bg-color: transparent; --el-table-tr-bg-color: transparent; --el-table-header-bg-color: #1a1a3a; --el-table-row-hover-bg-color: #1a1a3a; --el-table-text-color: #ccc; --el-table-header-text-color: #aaa; }
.el-dialog { background: #151530 !important; }
.el-form-item__label { color: #aaa !important; }
.el-input__wrapper { background: #1a1a3a !important; box-shadow: none !important; border: 1px solid #2a2a4a !important; }
.el-input__inner { color: #e0e0e0 !important; }
</style>
