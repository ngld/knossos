const fs = require('fs');
const postcss = require('postcss');

module.exports = (options, loaderContext) => {
  const blueprint = require.resolve('@blueprintjs/core/lib/css/blueprint.css');
  const root = postcss.parse(fs.readFileSync(blueprint, 'utf-8'));

  // remove all rules that apply to non-Blueprint elements (Blueprint uses bp4 as the prefix for all its classes)
  root.walkRules((rule, index) => {
    const selectors = rule.selectors.filter(
      (sel) => sel.indexOf('.bp4') > -1 || sel === 'to' || sel === 'from',
    );
    if (selectors.length < 1) {
      rule.remove();
    } else {
      rule.selectors = selectors;
    }
  });

  return {
    code: root.toString(),
    dependencies: [blueprint],
    cacheable: true,
  };
};
