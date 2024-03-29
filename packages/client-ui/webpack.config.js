const path = require('path');
const webpack = require('webpack');
const { merge } = require('webpack-merge');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const DuplicatePackageCheckerPlugin = require('duplicate-package-checker-webpack-plugin');
const UnpluginIcons = require('unplugin-icons/webpack');

// dev
const ReactRefreshPlugin = require('@pmmmwh/react-refresh-webpack-plugin');

// prod
const MiniCssExtractPlugin = require('mini-css-extract-plugin');
const { CleanWebpackPlugin } = require('clean-webpack-plugin');

const iconOptions = {
  defaultClass: 'icon',
  compiler: 'jsx',
  jsx: 'react',
};

module.exports = function (env, args) {
  const production = env.production;

  const cssLoaders = [
    production ? MiniCssExtractPlugin.loader : 'style-loader',
    {
      loader: 'css-loader',
      options: { esModule: false },
    },
    {
      loader: 'postcss-loader',
      options: {
        postcssOptions: {
          plugins: [
            ['postcss-import', {}],
            ['postcss-url', { url: 'rebase' }],
            ['postcss-nested-ancestors', {}],
            ['tailwindcss/nesting', {}],
            ['tailwindcss', {}],
            ['autoprefixer', {}],
            production ? ['cssnano', { preset: 'default' }] : null,
          ],
        },
      },
    },
  ];

  const baseConfig = {
    mode: production ? 'production' : 'development',
    devtool: production ? false : 'eval',
    entry: './src/index.tsx',
    output: {
      filename: production ? 'js/[name].[contenthash:7].js' : 'js/[name].js',
      chunkFilename: production ? 'js/[name].[contenthash:7].js' : 'js/[name].js',
      publicPath: '/',
    },
    optimization: {
      splitChunks: {
        chunks: 'all',
      },
      usedExports: true,
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        '@api': path.resolve(__dirname, '../api/api'),
      },
      extensions: ['.tsx', '.ts', '.jsx', '.js'],
    },
    module: {
      rules: [
        {
          test: /\.css$/,
          use: cssLoaders,
        },
        {
          test: /\.precss\.js$/,
          use: [...cssLoaders, 'val-loader'],
        },
        {
          test: /\.(png|jpe?g|gif)$/,
          loader: 'file-loader',
          options: {
            name: 'img/[name].[ext]',
          },
        },
        {
          test: /\.(ttf|eot|otf|woff2?|svg)$/,
          loader: 'file-loader',
          options: {
            name: 'fonts/[name].[ext]',
          },
        },
      ],
    },
  };

  const babelRule = (flavor) => ({
    output: {
      path: path.resolve(__dirname, 'dist', flavor),
    },
    module: {
      rules: [
        {
          test: /\.[tj]sx?$/,
          include: [path.resolve(__dirname, './src'), path.resolve(__dirname, '../api/api')],
          use: [
            {
              loader: 'babel-loader',
              options: {
                configFile: path.resolve(__dirname, `babel-${flavor}.config.js`),
              },
            },
            'astroturf/loader',
          ],
        },
        {
          test: /~icons%2F/,
          use: [
            {
              loader: 'babel-loader',
              options: {
                configFile: path.resolve(__dirname, `babel-${flavor}.config.js`),
              },
            },
          ],
        },
      ],
    },
  });

  const devConfig = {
    plugins: [
      new webpack.DefinePlugin({
        __DEV__: 'true',
        'process.env.NODE_ENV': '"development"',
        'process.env.BLUEPRINT_NAMESPACE': 'null',
        'process.env.REACT_APP_BLUEPRINT_NAMESPACE': 'null',
        global: 'window',
      }),
      new MiniCssExtractPlugin({
        filename: 'css/[name].css',
        chunkFilename: 'css/[id].css',
      }),
      new HtmlWebpackPlugin({
        template: path.resolve(__dirname, './html.ejs'),
      }),
      new ReactRefreshPlugin(),
      UnpluginIcons(iconOptions),
    ],
    devServer: {
      hot: true,
      historyApiFallback: {
        rewrites: [{ from: /./, to: '/index.html' }],
      },
      proxy: {
        '/twirp': 'http://localhost:8200/',
      },
    },
  };

  const prodConfig = {
    plugins: [
      new webpack.DefinePlugin({
        __DEV__: 'false',
        'process.env.NODE_ENV': '"production"',
        'process.env.BLUEPRINT_NAMESPACE': 'null',
        'process.env.REACT_APP_BLUEPRINT_NAMESPACE': 'null',
        global: 'window',
      }),
      new CleanWebpackPlugin(),
      new MiniCssExtractPlugin({
        filename: 'css/[name].[contenthash].css',
        chunkFilename: 'css/[id].[contenthash].css',
      }),
      new HtmlWebpackPlugin({
        template: path.resolve(__dirname, './html.ejs'),
      }),
      new DuplicatePackageCheckerPlugin(),
      UnpluginIcons(iconOptions),
    ],
  };

  return env.production
    ? merge(baseConfig, prodConfig, babelRule('prod'))
    : merge(baseConfig, devConfig, babelRule('dev'));
};
