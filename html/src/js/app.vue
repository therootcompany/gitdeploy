<template>
	<div class="app">
		<template v-if="ready">
			<div class="main">
				<router-view name="header" :user="user"></router-view>
				<router-view name="main" :user="user"></router-view>
			</div>
			<div class="x-footer">
				<div class="container is-fluid">
					<div class="columns">
						<div class="column x-l">
							<p>
								<a
									href="https://github.com/therootcompany/gitdeploy"
									target="_blank"
								>
									gitdeploy
								</a>
								v0.1.0
							</p>
						</div>
						<div class="column x-r">
							<p>
								<button v-if="signedIn" class="button is-small">
									Sign Out
								</button>
							</p>
						</div>
					</div>
				</div>
			</div>
		</template>
		<template v-else>
			<div class="page-loader">
				<span></span>
			</div>
		</template>
	</div>
</template>

<script>
var Vue = require('vue').default;
var axios = require('axios');

module.exports = {
	data: function () {
		return {
			user: {},
			ready: false
		};
	},
	methods: {
		verifySignedIn: function () {
			var _this = this;
			// TODO signin
			setTimeout(function () {
				_this.ready = true;
			}, 1000);
		}
	},
	watch: {
		$route: function (to, from) {
			// console.log('route change', to, from);
		}
	},
	computed: {
		signedIn: function () {
			return Object.keys(this.user).length > 0;
		}
	},
	created: function () {
		this.verifySignedIn();
	},
	router: require('./router.js')
};
</script>
