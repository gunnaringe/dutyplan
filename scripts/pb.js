// The page talks binary protobuf to the API. A script that fakes or changes an answer on its way to the page (ui-check,
// shots) writes it as JSON, the API's JSON with the proto's names, and this turns it into the binary answer. Node reads
// the generated TypeScript as it is (it strips the types).
const path = require('path');
const { fromBinary, fromJson, toBinary, toJson } = require('@bufbuild/protobuf');
const gen = require(path.join(__dirname, '../web/src/gen/dutyplan/v1/dutyplan_pb.ts'));

const names = { useProtoFieldName: true };
// the procedure a request calls («/dutyplan.v1.OrgService/GetOrg»)
const methodOf = (route) => {
  const [svc, name] = new URL(route.request().url()).pathname.split('/').slice(1);
  const m = Object.values(gen[svc.split('.').pop()].method).find((x) => x.name === name);
  if (!m) throw new Error('no procedure ' + svc + '/' + name);
  return m;
};
const binary = (m, json) => Buffer.from(toBinary(m.output, fromJson(m.output, json, names)));

// answer: fulfil the request with this answer (JSON)
exports.answer = (route, json) => route.fulfill({ status: 200, contentType: 'application/proto', body: binary(methodOf(route), json) });

// edit: the server's own answer, changed by edit(json) (a refusal goes through as it is)
exports.edit = async (route, edit) => {
  const m = methodOf(route);
  const res = await route.fetch();
  if (res.status() !== 200) return route.fulfill({ response: res });
  const json = toJson(m.output, fromBinary(m.output, await res.body()), { ...names, alwaysEmitImplicit: true });
  return route.fulfill({ response: res, body: binary(m, edit(json)) });
};
