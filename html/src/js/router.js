var Vue = require('vue').default;
var VueRouter = require('vue-router').default;
Vue.use(VueRouter);

var components = {
	navbar: require('./components/navbar.vue').default,
	dashboard: require('./components/dashboard.vue').default,
	notFound: require('./components/not-found.vue').default,
	jobs: require('./components/jobs.vue').default
};

var router = new VueRouter({
	routes: [
		{
			path: '/',
			components: {
				header: components.navbar,
				main: components.dashboard
			}
		},
		{
			path: '/jobs',
			components: {
				header: components.navbar,
				main: components.jobs
			}
		},
		{
			path: '*',
			components: {
				main: components.notFound
			}
		}
	]
});

module.exports = router;
