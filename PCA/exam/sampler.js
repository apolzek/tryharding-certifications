/* Sorteio do simulado PCA — usado pelo index.html (browser) e pelo test.sh (node).
 * O exam.py tem uma réplica em Python desta lógica (mesmos pesos e mesmo arredondamento). */
(function (root, factory) {
  if (typeof module === "object" && module.exports) module.exports = factory();
  else root.PCASampler = factory();
})(typeof self !== "undefined" ? self : this, function () {
  "use strict";

  // Pesos oficiais da PCA (ordem = desempate do arredondamento)
  var WEIGHTS = [
    ["PromQL", 0.28],
    ["Prometheus Fundamentals", 0.20],
    ["Observability Concepts", 0.18],
    ["Alerting and Dashboarding", 0.18],
    ["Instrumentation and Exporters", 0.16],
  ];
  var DOMAINS = WEIGHTS.map(function (w) { return w[0]; });

  // RNG com semente (mulberry32) — reprodutível nos testes
  function rng(seed) {
    var a = (seed >>> 0) || 1;
    return function () {
      a = (a + 0x6D2B79F5) >>> 0;
      var t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  function shuffle(arr, rand) {
    var a = arr.slice();
    for (var i = a.length - 1; i > 0; i--) {
      var j = Math.floor(rand() * (i + 1));
      var t = a[i]; a[i] = a[j]; a[j] = t;
    }
    return a;
  }

  // Cotas por domínio pelo método do maior resto. n=60 -> 17/12/11/11/9
  function quotas(n) {
    var q = {}, rest = [], used = 0;
    WEIGHTS.forEach(function (w, i) {
      var exact = n * w[1];
      q[w[0]] = Math.floor(exact);
      used += q[w[0]];
      rest.push([exact - Math.floor(exact), i, w[0]]);
    });
    rest.sort(function (x, y) { return (y[0] - x[0]) || (x[1] - y[1]); });
    for (var k = 0; used < n; k++, used++) q[rest[k % rest.length][2]] += 1;
    return q;
  }

  function filterLang(questions, lang) {
    return questions.filter(function (q) { return lang === "all" || !lang || q.lang === lang; });
  }

  // Sorteia n questões respeitando as cotas. Se faltar questão num domínio,
  // completa com os outros domínios (na ordem dos pesos) e avisa em .shortfall.
  function draw(questions, opts) {
    opts = opts || {};
    var n = opts.n || 60;
    var rand = opts.rand || (opts.seed != null ? rng(opts.seed) : Math.random);
    var pool = filterLang(questions, opts.lang || "all");
    var byDom = {};
    DOMAINS.forEach(function (d) { byDom[d] = shuffle(pool.filter(function (q) { return q.domain === d; }), rand); });
    var q = quotas(n), out = [], shortfall = 0;
    DOMAINS.forEach(function (d) {
      var take = Math.min(q[d], byDom[d].length);
      shortfall += q[d] - take;
      out = out.concat(byDom[d].splice(0, take));
    });
    var missing = shortfall;
    DOMAINS.forEach(function (d) {
      while (missing > 0 && byDom[d].length) { out.push(byDom[d].shift()); missing--; }
    });
    return { questions: shuffle(out, rand), quotas: q, shortfall: shortfall };
  }

  // Embaralha as alternativas; devolve a ordem (índices da ordem original)
  function optionOrder(rand) {
    return shuffle([0, 1, 2, 3], rand || Math.random);
  }

  // A explicação cita letras ("A) ...", "B e D ...")? Então, no treino, não embaralhamos as
  // alternativas (a explicação é mostrada logo em seguida e precisa bater com as letras).
  var LETTER_REF = /(^|[^\w`$])[A-D](\)|\s+e\s+[A-D]\b|,\s*[A-D]\b|\s)/;
  function citesLetters(q) { return LETTER_REF.test(q.explanation_pt || ""); }

  function score(questions, answers) {
    // answers: {id: "A".."D" (letra ORIGINAL)}; retorna geral + por domínio
    var by = {}, ok = 0;
    DOMAINS.forEach(function (d) { by[d] = { correct: 0, total: 0 }; });
    questions.forEach(function (qq) {
      var b = by[qq.domain] || (by[qq.domain] = { correct: 0, total: 0 });
      b.total++;
      if (answers[qq.id] === qq.answer) { b.correct++; ok++; }
    });
    var ratios = {};
    Object.keys(by).forEach(function (d) { if (by[d].total) ratios[d] = by[d].correct / by[d].total; });
    return { correct: ok, total: questions.length, score: questions.length ? ok / questions.length : 0, by_domain: ratios, detail: by };
  }

  return { WEIGHTS: WEIGHTS, DOMAINS: DOMAINS, PASS_MARK: 0.75, EXAM_MINUTES: 90,
           rng: rng, shuffle: shuffle, quotas: quotas, draw: draw, optionOrder: optionOrder,
           score: score, filterLang: filterLang, citesLetters: citesLetters };
});
