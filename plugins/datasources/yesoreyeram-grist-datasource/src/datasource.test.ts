import { DataSource } from './datasource';
import { GristQuery } from './types';

const makeDS = (docId?: string) =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-grist-datasource',
    name: 'Grist',
    jsonData: { baseURL: 'http://localhost:8484', docId },
  } as any);

describe('Grist DataSource', () => {
  it('reads docId from instance settings', () => {
    expect(makeDS('docABC').docId).toBe('docABC');
    expect(makeDS().docId).toBe('');
  });

  it('getDocs fetches via the docs resource', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ docs: [{ id: 'doc1', title: 'Sales' }] });
    (ds as any).getResource = getResource;

    const docs = await ds.getDocs();
    expect(getResource).toHaveBeenCalledWith('docs');
    expect(docs).toEqual([{ id: 'doc1', title: 'Sales' }]);
  });

  it('filterQuery requires a tableId for record/count queries', () => {
    expect(makeDS().filterQuery({ refId: 'A', queryType: 'records' } as GristQuery)).toBe(false);
    expect(makeDS().filterQuery({ refId: 'A', queryType: 'records', tableId: 't1' } as GristQuery)).toBe(true);
  });

  it('filterQuery requires a non-empty sql for sql queries', () => {
    expect(makeDS().filterQuery({ refId: 'A', queryType: 'sql' } as GristQuery)).toBe(false);
    expect(makeDS().filterQuery({ refId: 'A', queryType: 'sql', sql: '   ' } as GristQuery)).toBe(false);
    expect(makeDS().filterQuery({ refId: 'A', queryType: 'sql', sql: 'SELECT 1' } as GristQuery)).toBe(true);
  });

  it('getTables uses the configured doc when no docId is given', async () => {
    const ds = makeDS('docABC');
    const getResource = jest.fn().mockResolvedValue({ tables: [{ id: 't1', title: 'Users' }] });
    (ds as any).getResource = getResource;

    const tables = await ds.getTables();
    expect(getResource).toHaveBeenCalledWith('tables', undefined);
    expect(tables).toEqual([{ id: 't1', title: 'Users' }]);
  });

  it('getTables scopes to a doc when docId is given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({ tables: [] });
    (ds as any).getResource = getResource;

    await ds.getTables('docX');
    expect(getResource).toHaveBeenCalledWith('tables', { docId: 'docX' });
  });

  it('getFields fetches a table fields and forwards docId when given', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      fields: [
        { title: 'Name', type: 'Text' },
        { title: 'Age', type: 'Numeric' },
      ],
    });
    (ds as any).getResource = getResource;

    const fields = await ds.getFields('t1', 'docX');
    expect(getResource).toHaveBeenCalledWith('fields', { tableId: 't1', docId: 'docX' });
    expect(fields).toEqual([
      { title: 'Name', type: 'Text' },
      { title: 'Age', type: 'Numeric' },
    ]);
  });

  it('getFields returns empty without tableId', async () => {
    const ds = makeDS();
    const getResource = jest.fn();
    (ds as any).getResource = getResource;

    await expect(ds.getFields('')).resolves.toEqual([]);
    expect(getResource).not.toHaveBeenCalled();
  });
});
