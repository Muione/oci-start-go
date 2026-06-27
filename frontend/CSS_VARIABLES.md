# CSS 变量和设计令牌参考

## 快速参考

### 颜色变量

#### 主要颜色
```css
--primary-color: #0066ff;        /* 主蓝色 */
--primary-light: #3385ff;        /* 浅蓝色 */
--primary-dark: #0052cc;         /* 深蓝色 */
```

#### 状态颜色
```css
--success-color: #10b981;        /* 成功绿 */
--warning-color: #f59e0b;        /* 警告橙 */
--danger-color: #ef4444;         /* 危险红 */
--info-color: #0066ff;           /* 信息蓝 */
--secondary-color: #00bcd4;      /* 辅助青 */
--secondary-light: #4dd0e1;      /* 浅青 */
```

#### 背景颜色
```css
--bg-primary: #ffffff;           /* 主背景 */
--bg-secondary: #f8fafc;         /* 次背景 */
--bg-tertiary: #f1f5f9;          /* 三级背景 */
--bg-dark: #001529;              /* 深色背景 */
```

#### 文本颜色
```css
--text-primary: #1e293b;         /* 主文本 */
--text-secondary: #64748b;       /* 次文本 */
--text-tertiary: #94a3b8;        /* 三级文本 */
```

#### 边框和分割线
```css
--border-color: #e2e8f0;         /* 主边框 */
--border-color-light: #f1f5f9;   /* 浅边框 */
```

### 阴影变量

```css
--shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
--shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
--shadow-lg: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
--shadow-xl: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
```

### 字体变量

```css
--font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen', 
               'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue', sans-serif;
--font-mono: 'Menlo', 'Monaco', 'Courier New', monospace;
```

## 使用示例

### 在 Vue 组件中使用变量

```vue
<style scoped>
.card {
  background: var(--bg-primary);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  box-shadow: var(--shadow-md);
  border-radius: 12px;
  padding: 24px;
}

.card:hover {
  box-shadow: var(--shadow-lg);
}

.text-secondary {
  color: var(--text-secondary);
}
</style>
```

### 创建渐变

```css
/* 主渐变 */
.gradient-primary {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-light));
}

/* 成功渐变 */
.gradient-success {
  background: linear-gradient(135deg, var(--success-color), #34d399);
}

/* 渐变文字 */
.gradient-text {
  background: linear-gradient(135deg, var(--primary-color), var(--secondary-color));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
```

## 预定义的样式类

### 卡片样式
```css
.stat-card {
  background: linear-gradient(135deg, #ffffff, #f8fafc);
  border-radius: 16px;
  border: 1px solid rgba(0, 102, 255, 0.1);
  box-shadow: var(--shadow-sm);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.stat-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-6px);
  border-color: rgba(0, 102, 255, 0.2);
}
```

### 按钮样式
```css
/* 主按钮 */
.btn-primary {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-light));
  border: none;
  border-radius: 8px;
  font-weight: 600;
  transition: all 0.3s ease;
}

.btn-primary:hover {
  background: linear-gradient(135deg, var(--primary-dark), var(--primary-color));
  box-shadow: 0 8px 16px rgba(0, 102, 255, 0.3);
  transform: translateY(-1px);
}
```

### 标签样式
```css
.tag-primary {
  background: rgba(0, 102, 255, 0.1);
  color: var(--primary-color);
  border-radius: 8px;
  border: none;
  font-weight: 600;
  padding: 6px 12px;
}

.tag-success {
  background: rgba(16, 185, 129, 0.1);
  color: var(--success-color);
}
```

## 响应式设计断点

```css
/* 平板和手机 */
@media (max-width: 768px) {
  /* 样式 */
}

/* 小屏幕手机 */
@media (max-width: 480px) {
  /* 样式 */
}

/* 超大屏幕 */
@media (min-width: 1920px) {
  /* 样式 */
}
```

## 动画和过渡

### 预定义动画

```css
/* 淡入 */
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.fade-in {
  animation: fadeIn 0.3s ease-out;
}

/* 滑入 */
@keyframes slideInLeft {
  from { opacity: 0; transform: translateX(-20px); }
  to { opacity: 1; transform: translateX(0); }
}

.slide-in-left {
  animation: slideInLeft 0.3s ease-out;
}
```

### 标准过渡

所有交互元素使用标准过渡时间函数：

```css
transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
```

## Element Plus 组件自定义

### 按钮
```vue
<el-button type="primary">主按钮</el-button>
<!-- 应用渐变背景和悬停效果 -->
```

### 输入框
```vue
<el-input v-model="value" placeholder="输入内容" />
<!-- 应用现代圆角和焦点样式 -->
```

### 表格
```vue
<el-table :data="rows">
  <!-- 应用自定义表头和行样式 -->
</el-table>
```

### 标签
```vue
<el-tag type="primary">标签</el-tag>
<!-- 应用圆角和半透明背景 -->
```

## 主题定制指南

### 修改主色调

编辑 `src/style.css` 中的 `:root` 部分：

```css
:root {
  /* 将这些值改为你的品牌色 */
  --primary-color: #0066ff;
  --primary-light: #3385ff;
  --primary-dark: #0052cc;
}
```

### 修改间距系统

所有间距都使用具体的 px 值，可以统一调整：

```css
/* 卡片内间距 */
padding: 24px;  /* 改为其他值 */

/* 组件间距 */
gap: 20px;      /* 改为其他值 */

/* 元素间距 */
margin-bottom: 28px;  /* 改为其他值 */
```

### 修改圆角

```css
/* 卡片 */
border-radius: 16px;  /* 改为其他值 */

/* 按钮 */
border-radius: 8px;   /* 改为其他值 */

/* 输入框 */
border-radius: 8px;   /* 改为其他值 */
```

### 修改阴影

```css
box-shadow: var(--shadow-lg);  /* 选择其他阴影 */
```

## 暗色模式支持

所有样式都支持暗色模式偏好：

```css
@media (prefers-color-scheme: dark) {
  :root {
    --bg-primary: #1e293b;
    --bg-secondary: #0f172a;
    --text-primary: #f1f5f9;
    /* 其他暗色变量 */
  }
}
```

## 性能最佳实践

- ✅ 使用 CSS 变量避免重复
- ✅ 使用 `transform` 和 `opacity` 进行动画
- ✅ 避免过度使用 `box-shadow`
- ✅ 使用 `will-change` 优化重型动画
- ✅ 懒加载背景图片

## 常见问题

**Q: 如何修改按钮的默认颜色？**
A: 在你的 Vue 组件中覆盖 Element Plus 的默认颜色：
```css
:deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--primary-color), var(--primary-light));
}
```

**Q: 如何添加深色模式？**
A: 页面已支持系统深色模式偏好，用户可以在操作系统设置中切换。

**Q: 如何自定义卡片的阴影？**
A: 直接修改 CSS 变量或在组件中覆盖：
```css
box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
```

## 有用的链接

- [CSS Variables MDN](https://developer.mozilla.org/en-US/docs/Web/CSS/--*)
- [CSS Grid 响应式设计](https://developer.mozilla.org/en-US/docs/Web/CSS/CSS_Grid_Layout)
- [Element Plus 文档](https://element-plus.org/)
- [现代 CSS 特性](https://developer.mozilla.org/en-US/docs/Web/CSS)
