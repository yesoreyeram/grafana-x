import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import { Button, IconButton, InlineField, InlineFieldRow, InlineSwitch, Input, MultiSelect, RadioButtonGroup, Select, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { FieldInfo, GristDataSourceOptions, GristQuery, GristQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, GristQuery, GristDataSourceOptions>;

const LABEL_WIDTH = 20;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<GristQueryType>> = [
  { label: 'Records', value: 'records', description: 'Return matching records' },
  { label: 'Count', value: 'count', description: 'Return the number of matching records' },
  { label: 'SQL', value: 'sql', description: 'Run a raw read-only Grist SQL SELECT' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

export function tableChangePatch(tableId: string, changingTable: boolean): Partial<GristQuery> {
  if (!changingTable) {
    return { tableId };
  }
  return { tableId, filterTree: '', sort: '', fields: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { tableId, sort, fields, filterTree, limit } = query;
  const queryType = query.queryType ?? 'records';
  const isCount = queryType === 'count';
  const isSQL = queryType === 'sql';
  const configuredDocId = datasource.docId;
  const showDocPicker = !configuredDocId;

  const selectedDocId = showDocPicker ? (query.docId ?? '') : configuredDocId;
  const docId = selectedDocId;

  const [docs, setDocs] = useState<Array<SelectableValue<string>>>([]);
  const [loadingDocs, setLoadingDocs] = useState(false);
  const [docsError, setDocsError] = useState<string | undefined>();

  const [tables, setTables] = useState<Array<SelectableValue<string>>>([]);
  const [loadingTables, setLoadingTables] = useState(false);
  const [tablesError, setTablesError] = useState<string | undefined>();

  const [fieldList, setFieldList] = useState<FieldInfo[]>([]);
  const [fieldOptions, setFieldOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingFields, setLoadingFields] = useState(false);
  const [fieldsError, setFieldsError] = useState<string | undefined>();

  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load the list of docs (only when no doc id is configured).
  useEffect(() => {
    if (!showDocPicker) {
      return;
    }
    let cancelled = false;
    setLoadingDocs(true);
    setDocsError(undefined);
    datasource
      .getDocs()
      .then((res) => {
        if (!cancelled) {
          setDocs(res.map((d) => ({ label: d.title, value: d.id, description: d.id })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setDocsError(err?.data?.error ?? err?.message ?? 'Failed to load docs');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingDocs(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [showDocPicker, datasource]);

  // Load the selected doc's tables.
  useEffect(() => {
    let cancelled = false;
    if (showDocPicker && !docId) {
      setTables([]);
      setTablesError(undefined);
      return;
    }
    setLoadingTables(true);
    setTablesError(undefined);
    datasource
      .getTables(docId || undefined)
      .then((res) => {
        if (cancelled) {
          return;
        }
        setTables(res.map((t) => ({ label: t.title, value: t.id, description: t.id })));
      })
      .catch((err) => {
        if (!cancelled) {
          setTablesError(err?.data?.error ?? err?.message ?? 'Failed to load tables');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingTables(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [docId, showDocPicker, datasource]);

  // Load the selected table's fields.
  useEffect(() => {
    let cancelled = false;
    if (!tableId) {
      setFieldList([]);
      setFieldOptions([]);
      setFieldsError(undefined);
      return;
    }
    setLoadingFields(true);
    setFieldsError(undefined);
    datasource
      .getFields(tableId, docId || undefined)
      .then((res) => {
        if (!cancelled) {
          setFieldList(res);
          setFieldOptions(res.map((f) => ({ label: f.title, value: f.title, description: f.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setFieldsError(err?.data?.error ?? err?.message ?? 'Failed to load fields');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingFields(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [tableId, docId, datasource]);

  const update = useCallback(
    (patch: Partial<GristQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: GristQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onDocSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    if (id === selectedDocId) {
      return;
    }
    setFilterRoot(emptyRootGroup());
    setSortItems([]);
    update({ docId: id, ...tableChangePatch('', true) });
    onRunQuery();
  };

  const selectedDoc: SelectableValue<string> | null = selectedDocId
    ? (docs.find((d) => d.value === selectedDocId) ?? { label: selectedDocId, value: selectedDocId })
    : null;

  const onTableIdSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingTable = id !== tableId;
    if (changingTable) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(tableChangePatch(id, changingTable));
    onRunQuery();
  };

  const selectedTable: SelectableValue<string> | null = tableId
    ? (tables.find((t) => t.value === tableId) ?? { label: tableId, value: tableId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onSqlChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    update({ sql: e.target.value });
  };

  const onFilterChange = (root: FilterGroup) => {
    setFilterRoot(root);
    update({ filterTree: stringifyFilterTree(root) });
    onRunQuery();
  };

  const applySort = (items: SortItem[], run: boolean) => {
    setSortItems(items);
    const serialized = serializeSort(items);
    if (serialized !== (sort ?? '')) {
      update({ sort: serialized });
      if (run) {
        onRunQuery();
      }
    }
  };

  const onAddSort = () => {
    setSortItems([...sortItems, { field: '', direction: 'asc' }]);
  };

  const onRemoveSort = (index: number) => {
    applySort(
      sortItems.filter((_, i) => i !== index),
      true
    );
  };

  const onSortFieldChange = (index: number, value: SelectableValue<string> | null) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, field: value?.value ?? '' } : item));
    applySort(next, true);
  };

  const onSortDirectionChange = (index: number, direction: SortDirection) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, direction } : item));
    applySort(next, true);
  };

  const selectedFields: Array<SelectableValue<string>> = (fields ?? '')
    .split(',')
    .map((f) => f.trim())
    .filter((f) => f.length > 0)
    .map((f) => fieldOptions.find((o) => o.value === f) ?? { label: f, value: f });

  const onFieldsSelect = (values: Array<SelectableValue<string>>) => {
    const list = values.map((v) => v.value).filter((v): v is string => !!v);
    update({ fields: list.join(',') });
    onRunQuery();
  };

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Records returns rows; Count returns the number of matching records (via SQL COUNT(*)); SQL runs a raw read-only SELECT."
        >
          <RadioButtonGroup<GristQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      {showDocPicker && (
        <div className="gf-form">
          <InlineField
            label="Document"
            labelWidth={LABEL_WIDTH}
            tooltip="Select a Grist document. The list is fetched via the API."
            error={docsError}
            invalid={!!docsError}
            required
          >
            <Select<string>
              width={40}
              isClearable
              isLoading={loadingDocs}
              options={docs}
              value={selectedDoc}
              onChange={onDocSelect}
              allowCustomValue
              placeholder="Select document"
              noOptionsMessage="No documents found"
            />
          </InlineField>
        </div>
      )}

      {isSQL ? (
        <div className="gf-form" style={{ alignItems: 'flex-start' }}>
          <InlineField
            label="SQL"
            labelWidth={LABEL_WIDTH}
            tooltip="A single read-only SQL SELECT statement run against the document's SQLite database (no trailing semicolon). Date/DateTime columns return epoch seconds."
            grow
          >
            <TextArea
              rows={6}
              value={query.sql ?? ''}
              placeholder={'SELECT * FROM "Table1" WHERE "Col" > 10'}
              onChange={onSqlChange}
              onBlur={onRunQuery}
            />
          </InlineField>
        </div>
      ) : (
        <>
          <InlineFieldRow>
            <InlineField
              label="Table"
              labelWidth={LABEL_WIDTH}
              tooltip="Select a Grist table. The list is fetched for the selected doc. You can also type a table id or name manually."
              error={tablesError}
              invalid={!!tablesError}
              required
            >
              <Select<string>
                width={40}
                isClearable
                isLoading={loadingTables}
                options={tables}
                value={selectedTable}
                onChange={onTableIdSelect}
                allowCustomValue
                disabled={showDocPicker && !docId}
                placeholder={showDocPicker && !docId ? 'Select a doc first' : 'Select table'}
                noOptionsMessage="No tables found"
              />
            </InlineField>
          </InlineFieldRow>

          {!isCount && (
            <div className="gf-form">
              <InlineField
                label="Fields"
                labelWidth={LABEL_WIDTH}
                tooltip="Fields to return. Leave empty to return all fields. Selecting fields runs the query via SQL (the records endpoint has no projection)."
                error={fieldsError}
                invalid={!!fieldsError}
              >
                <MultiSelect<string>
                  width={40}
                  isLoading={loadingFields}
                  options={fieldOptions}
                  value={selectedFields}
                  onChange={onFieldsSelect}
                  allowCustomValue
                  placeholder={tableId ? 'All fields' : 'Select a table first'}
                  disabled={!tableId}
                  noOptionsMessage="No fields found"
                />
              </InlineField>
            </div>
          )}

          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Filters"
              labelWidth={LABEL_WIDTH}
              tooltip="Filter records. Simple equality/membership filters use the fast records endpoint; richer operators (>, <, contains, !=, OR, groups) compile to parameterized SQL server-side. Operators adapt to each field's type."
            >
              <FilterEditor group={filterRoot} fields={fieldList} disabled={!tableId} onChange={onFilterChange} />
            </InlineField>
          </div>

          {!isCount && (
            <>
              <div className="gf-form" style={{ alignItems: 'flex-start' }}>
                <InlineField
                  label="Sort"
                  labelWidth={LABEL_WIDTH}
                  tooltip="Order results by one or more fields. Rows are applied in order."
                >
                  <div>
                    {sortItems.map((item, index) => (
                      <InlineFieldRow key={index} style={{ marginBottom: 4, alignItems: 'center' }}>
                        <Select<string>
                          width={28}
                          options={fieldOptions}
                          value={
                            item.field
                              ? (fieldOptions.find((o) => o.value === item.field) ?? {
                                  label: item.field,
                                  value: item.field,
                                })
                              : null
                          }
                          onChange={(v) => onSortFieldChange(index, v)}
                          allowCustomValue
                          placeholder="Select field"
                          disabled={!tableId}
                          noOptionsMessage="No fields found"
                        />
                        <RadioButtonGroup<SortDirection>
                          options={DIRECTION_OPTIONS}
                          value={item.direction}
                          onChange={(v) => onSortDirectionChange(index, v)}
                        />
                        <IconButton
                          name="trash-alt"
                          tooltip="Remove sort"
                          aria-label="Remove sort"
                          onClick={() => onRemoveSort(index)}
                        />
                      </InlineFieldRow>
                    ))}
                    <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!tableId}>
                      Add sort
                    </Button>
                  </div>
                </InlineField>
              </div>

              <InlineFieldRow>
                <div className="gf-form">
                  <InlineField
                    label="Limit"
                    labelWidth={LABEL_WIDTH}
                    tooltip="Maximum number of records. 0 returns all rows (the Grist records endpoint has no offset pagination)."
                  >
                    <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
                  </InlineField>
                </div>
              </InlineFieldRow>
            </>
          )}
        </>
      )}
      <InlineFieldRow>
        <InlineField
          label="Hide system fields"
          labelWidth={LABEL_WIDTH}
          tooltip="Hide metadata-style columns (id, created_at/updated_at, underscore-prefixed names, etc.) from the returned frame."
        >
          <InlineSwitch
            value={!!query.hideSystemFields}
            onChange={(e) => {
              update({ hideSystemFields: e.currentTarget.checked });
              onRunQuery();
            }}
          />
        </InlineField>
      </InlineFieldRow>

    </div>
  );
}
