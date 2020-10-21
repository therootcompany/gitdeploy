<template>
	<div class="sites">
		<div class="container is-fluid">
			<div class="title">
				<h2 class="has-text-weight-bold">Sites</h2>
			</div>
			<div class="block site" v-for="s in sites">
				<div class="title">
					<h3 class="has-text-weight-bold">{{ s.id }}</h3>
					... promote
					<p>
						<button
							class="button is-primary"
							@click="promote($event, s.ref_name)"
						>
							<span>Promote</span>&nbsp;<b>{{ s.ref_name }}</b>
						</button>
					</p>
				</div>
			</div>
		</div>
	</div>
</template>

<script>
var axios = require('axios');

module.exports = {
	name: 'sites',
	data: function () {
		return {
			sites: []
		};
	},
	methods: {
		getSites: function () {
			var _this = this;

			return axios.get('/api/admin/sites').then(function (resp) {
				_this.sites = resp.data;
			});
		},
		promote: function (ev, ref_name) {
			if (ev) {
				ev.preventDefault();
			}

			if (window.confirm('Are you sure you want to promote master?')) {
				axios.post('/api/admin/promote', {
					clone_url: 'https://...',
					ref_name: ref_name
				});
			}
		}
	},
	created: function () {
		var _this = this;

		_this.getSites();
	}
};
</script>
