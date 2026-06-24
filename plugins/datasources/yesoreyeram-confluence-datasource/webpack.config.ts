import type { Configuration } from 'webpack';
import * as path from 'path';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import ForkTsCheckerWebpackPlugin from 'fork-ts-checker-webpack-plugin';
import ESLintPlugin from 'eslint-webpack-plugin';
// eslint-disable-next-line @typescript-eslint/no-var-requires
const ReplaceInFileWebpackPlugin = require('replace-in-file-webpack-plugin');
// eslint-disable-next-line @typescript-eslint/no-var-requires
const LiveReloadPlugin = require('webpack-livereload-plugin');
import TsconfigPathsPlugin from 'tsconfig-paths-webpack-plugin';

import { getPackageJson, getPluginJson, getEntries } from './webpack.utils';

const pluginJson = getPluginJson();

const config = async (env: Record<string, unknown>): Promise<Configuration> => {
  const baseConfig: Configuration = {
    cache: { type: 'filesystem' },
    context: path.join(process.cwd(), 'src'),
    devtool: env.production ? 'source-map' : 'eval-source-map',
    entry: await getEntries(),
    externals: [
      'lodash',
      'jquery',
      'moment',
      'slate',
      'emotion',
      '@emotion/react',
      '@emotion/css',
      'prismjs',
      'slate-plain-serializer',
      '@grafana/slate-react',
      'react',
      'react-dom',
      'react-redux',
      'redux',
      'rxjs',
      'react-router',
      'react-router-dom',
      'd3',
      'angular',
      '@grafana/ui',
      '@grafana/runtime',
      '@grafana/data',
      // Anything imported as @grafana/* is provided by Grafana at runtime
      ({ request }, callback) => {
        const prefix = 'grafana/';
        const hasPrefix = (r: string) => r.indexOf(prefix) === 0;
        const stripPrefix = (r: string) => r.substring(prefix.length);
        if (request && hasPrefix(request)) {
          return callback(undefined, stripPrefix(request));
        }
        callback();
      },
    ],
    mode: env.production ? 'production' : 'development',
    module: {
      rules: [
        {
          exclude: /(node_modules)/,
          test: /\.[tj]sx?$/,
          use: {
            loader: 'swc-loader',
            options: {
              jsc: {
                baseUrl: path.resolve(process.cwd(), 'src'),
                target: 'es2015',
                loose: false,
                parser: { syntax: 'typescript', tsx: true, decorators: false, dynamicImport: true },
              },
            },
          },
        },
        {
          test: /\.css$/,
          use: ['style-loader', 'css-loader'],
        },
        {
          test: /\.s[ac]ss$/,
          use: ['style-loader', 'css-loader', 'sass-loader'],
        },
        {
          test: /\.(png|jpe?g|gif|svg)$/,
          type: 'asset/resource',
          generator: { publicPath: `public/plugins/${pluginJson.id}/img/`, outputPath: 'img/' },
        },
        {
          test: /\.(woff|woff2|eot|ttf|otf)(\?v=\d+\.\d+\.\d+)?$/,
          type: 'asset/resource',
          generator: { publicPath: `public/plugins/${pluginJson.id}/fonts/`, outputPath: 'fonts/' },
        },
      ],
    },
    output: {
      clean: { keep: new RegExp(`(.*?_(amd64|arm(64)?)(.exe)?|go_plugin_build_manifest)`) },
      filename: '[name].js',
      library: { type: 'amd' },
      path: path.resolve(process.cwd(), 'dist'),
      publicPath: `public/plugins/${pluginJson.id}/`,
      uniqueName: pluginJson.id,
    },
    plugins: [
      new CopyWebpackPlugin({
        patterns: [
          { from: 'plugin.json', to: '.' },
          { from: '../README.md', to: '.', force: true, noErrorOnMissing: true },
          { from: '../LICENSE', to: '.', noErrorOnMissing: true },
          { from: '../CHANGELOG.md', to: '.', force: true, noErrorOnMissing: true },
          { from: '**/*.json', to: '.', noErrorOnMissing: true },
          { from: 'img/**/*', to: '.', noErrorOnMissing: true },
          { from: '**/*.svg', to: '.', noErrorOnMissing: true },
        ],
      }),
      new ReplaceInFileWebpackPlugin([
        {
          dir: 'dist',
          files: ['plugin.json', 'README.md'],
          rules: [
            { search: /\%VERSION\%/g, replace: getPackageJson().version },
            { search: /\%TODAY\%/g, replace: new Date().toISOString().substring(0, 10) },
          ],
        },
      ]),
      new ForkTsCheckerWebpackPlugin({
        async: Boolean(env.development),
        issue: {
          include: [{ file: '**/*.{ts,tsx}' }],
          exclude: [{ file: '**/*.test.{ts,tsx}' }],
        },
        typescript: { configFile: path.join(process.cwd(), 'tsconfig.json') },
      }),
      new ESLintPlugin({ extensions: ['.ts', '.tsx'], lintDirtyModulesOnly: Boolean(env.development) }),
      ...(env.development ? [new LiveReloadPlugin()] : []),
    ],
    resolve: {
      extensions: ['.js', '.jsx', '.ts', '.tsx'],
      plugins: [new TsconfigPathsPlugin({ extensions: ['.js', '.jsx', '.ts', '.tsx'] })],
    },
  };

  return baseConfig;
};

export default config;
