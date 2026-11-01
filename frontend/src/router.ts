import { createRouter, createWebHashHistory, RouteRecordRaw } from 'vue-router';

const WelcomePage = () => import('./pages/WelcomePage.vue');
const AuthLayout = () => import('./layouts/AuthLayout.vue');
const WorkspaceLayout = () => import('./layouts/WorkspaceLayout.vue');

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: AuthLayout,
    children: [
      {
        path: '',
        component: WelcomePage,
        name: 'Welcome',
      },
    ],
  },
  {
    path: '/workspace',
    component: WorkspaceLayout,
    name: 'WorkSpace',
    // children: [
    //   {
    //     path: '',
    //     component: WorkSpacePage,
    //     name: 'WorkSpace',
    //   },
    // ],
  },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
});

export default router;
