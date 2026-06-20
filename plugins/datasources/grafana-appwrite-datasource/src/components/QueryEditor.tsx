import React, { ChangeEvent, useCallback, useEffect, useState } from 'react';
import {
  InlineField,
  InlineFieldRow,
  Input,
  Select,
  MultiSelect,
  IconButton,
  RadioButtonGroup,
  Button,
  TextArea,
  InlineSwitch,
} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import { AttributeInfo, AppwriteDataSourceOptions, AppwriteQuery, AppwriteQueryType } from '../types';
import { parseSort, serializeSort, SortDirection, SortItem } from '../sort';
import { emptyRootGroup, FilterGroup, parseFilterTree, stringifyFilterTree } from '../filter';
import { FilterEditor } from './FilterEditor';

type Props = QueryEditorProps<DataSource, AppwriteQuery, AppwriteDataSourceOptions>;

const LABEL_WIDTH = 16;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<AppwriteQueryType>> = [
  { label: 'Documents', value: 'documents', description: 'Return matching documents' },
  { label: 'Count', value: 'count', description: 'Return the number of matching documents' },
];

const DIRECTION_OPTIONS: Array<SelectableValue<SortDirection>> = [
  { label: 'Asc', value: 'asc' },
  { label: 'Desc', value: 'desc' },
];

/**
 * Build the query patch for a collection selection. When the collection changes,
 * the collection-dependent options (filters, sort, attributes) are cleared
 * because they reference attributes that no longer exist in the new collection.
 */
export function collectionChangePatch(collectionId: string, changingCollection: boolean): Partial<AppwriteQuery> {
  if (!changingCollection) {
    return { collectionId };
  }
  return { collectionId, filterTree: '', sort: '', attributes: '' };
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { collectionId, sort, attributes, filterTree, rawQueries, limit, hideSystemFields } = query;
  const queryType = query.queryType ?? 'documents';
  const isCount = queryType === 'count';
  const configuredDatabaseId = datasource.databaseId;
  // When no database id is configured on the datasource, the user picks a
  // database here to scope the collection list.
  const showDatabasePicker = !configuredDatabaseId;

  // The database to list collections from: the picked one, else the configured one.
  const selectedDatabaseId = showDatabasePicker ? (query.databaseId ?? '') : configuredDatabaseId;
  const databaseId = selectedDatabaseId;

  const [databases, setDatabases] = useState<Array<SelectableValue<string>>>([]);
  const [loadingDatabases, setLoadingDatabases] = useState(false);
  const [databasesError, setDatabasesError] = useState<string | undefined>();

  const [collections, setCollections] = useState<Array<SelectableValue<string>>>([]);
  const [loadingCollections, setLoadingCollections] = useState(false);
  const [collectionsError, setCollectionsError] = useState<string | undefined>();

  const [attributeList, setAttributeList] = useState<AttributeInfo[]>([]);
  const [attributeOptions, setAttributeOptions] = useState<Array<SelectableValue<string>>>([]);
  const [loadingAttributes, setLoadingAttributes] = useState(false);
  const [attributesError, setAttributesError] = useState<string | undefined>();

  // Sort is persisted as a JSON array of {attribute, direction} but edited as rows.
  const [sortItems, setSortItems] = useState<SortItem[]>(() => parseSort(sort));

  useEffect(() => {
    if (serializeSort(sortItems) !== (sort ?? '')) {
      setSortItems(parseSort(sort));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sort]);

  // Structured filter tree edited via the filter builder. Persisted as JSON
  // (filterTree); the query strings are built server-side.
  const [filterRoot, setFilterRoot] = useState<FilterGroup>(() => parseFilterTree(filterTree));

  useEffect(() => {
    if (stringifyFilterTree(filterRoot) !== (filterTree ?? '')) {
      setFilterRoot(parseFilterTree(filterTree));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filterTree]);

  // Load the list of databases (only when no database id is configured).
  useEffect(() => {
    if (!showDatabasePicker) {
      return;
    }
    let cancelled = false;
    setLoadingDatabases(true);
    setDatabasesError(undefined);
    datasource
      .getDatabases()
      .then((res) => {
        if (!cancelled) {
          setDatabases(res.map((d) => ({ label: d.name, value: d.id, description: d.id })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setDatabasesError(err?.data?.error ?? err?.message ?? 'Failed to load databases');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingDatabases(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [showDatabasePicker, datasource]);

  // Load the selected database's collections.
  useEffect(() => {
    let cancelled = false;
    if (showDatabasePicker && !databaseId) {
      setCollections([]);
      setCollectionsError(undefined);
      return;
    }
    setLoadingCollections(true);
    setCollectionsError(undefined);
    datasource
      .getCollections(databaseId || undefined)
      .then((res) => {
        if (cancelled) {
          return;
        }
        setCollections(res.map((c) => ({ label: c.name, value: c.id, description: c.id })));
      })
      .catch((err) => {
        if (!cancelled) {
          setCollectionsError(err?.data?.error ?? err?.message ?? 'Failed to load collections');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingCollections(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [databaseId, showDatabasePicker, datasource]);

  // Load the selected collection's attributes for the multi-select and filters.
  useEffect(() => {
    let cancelled = false;
    if (!collectionId) {
      setAttributeList([]);
      setAttributeOptions([]);
      setAttributesError(undefined);
      return;
    }
    setLoadingAttributes(true);
    setAttributesError(undefined);
    datasource
      .getAttributes(collectionId, databaseId || undefined)
      .then((res) => {
        if (!cancelled) {
          setAttributeList(res);
          setAttributeOptions(res.map((a) => ({ label: a.key, value: a.key, description: a.type })));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setAttributesError(err?.data?.error ?? err?.message ?? 'Failed to load attributes');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingAttributes(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [collectionId, databaseId, datasource]);

  const update = useCallback(
    (patch: Partial<AppwriteQuery>) => {
      onChange({ ...query, ...patch });
    },
    [onChange, query]
  );

  const onQueryTypeChange = (value: AppwriteQueryType) => {
    update({ queryType: value });
    onRunQuery();
  };

  const onDatabaseSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    if (id === selectedDatabaseId) {
      return;
    }
    // Changing database invalidates the current collection and its dependents.
    setFilterRoot(emptyRootGroup());
    setSortItems([]);
    update({ databaseId: id, ...collectionChangePatch('', true) });
    onRunQuery();
  };

  const selectedDatabase: SelectableValue<string> | null = selectedDatabaseId
    ? (databases.find((d) => d.value === selectedDatabaseId) ?? {
        label: selectedDatabaseId,
        value: selectedDatabaseId,
      })
    : null;

  const onCollectionSelect = (value: SelectableValue<string> | null) => {
    const id = value?.value ?? '';
    const changingCollection = id !== collectionId;
    if (changingCollection) {
      setFilterRoot(emptyRootGroup());
      setSortItems([]);
    }
    update(collectionChangePatch(id, changingCollection));
    onRunQuery();
  };

  const selectedCollection: SelectableValue<string> | null = collectionId
    ? (collections.find((c) => c.value === collectionId) ?? { label: collectionId, value: collectionId })
    : null;

  const onLimitChange = (e: ChangeEvent<HTMLInputElement>) => {
    const n = parseInt(e.target.value, 10);
    update({ limit: isNaN(n) ? 0 : n });
  };

  const onFilterChange = (root: FilterGroup) => {
    setFilterRoot(root);
    update({ filterTree: stringifyFilterTree(root) });
    onRunQuery();
  };

  const onRawQueriesChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    update({ rawQueries: e.currentTarget.value });
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
    setSortItems([...sortItems, { attribute: '', direction: 'asc' }]);
  };

  const onRemoveSort = (index: number) => {
    applySort(
      sortItems.filter((_, i) => i !== index),
      true
    );
  };

  const onSortAttributeChange = (index: number, value: SelectableValue<string> | null) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, attribute: value?.value ?? '' } : item));
    applySort(next, true);
  };

  const onSortDirectionChange = (index: number, direction: SortDirection) => {
    const next = sortItems.map((item, i) => (i === index ? { ...item, direction } : item));
    applySort(next, true);
  };

  // `attributes` is persisted as a comma-separated string for the select query.
  const selectedAttributes: Array<SelectableValue<string>> = (attributes ?? '')
    .split(',')
    .map((a) => a.trim())
    .filter((a) => a.length > 0)
    .map((a) => attributeOptions.find((o) => o.value === a) ?? { label: a, value: a });

  const onAttributesSelect = (values: Array<SelectableValue<string>>) => {
    const list = values.map((v) => v.value).filter((v): v is string => !!v);
    update({ attributes: list.join(',') });
    onRunQuery();
  };

  const onHideSystemFieldsChange = (e: ChangeEvent<HTMLInputElement>) => {
    update({ hideSystemFields: e.currentTarget.checked });
    onRunQuery();
  };

  const usingRawQueries = !!rawQueries && rawQueries.trim().length > 0;

  return (
    <div>
      <div className="gf-form">
        <InlineField
          label="Query type"
          labelWidth={LABEL_WIDTH}
          tooltip="Documents returns rows; Count returns the number of matching documents (respecting filters)."
        >
          <RadioButtonGroup<AppwriteQueryType>
            options={QUERY_TYPE_OPTIONS}
            value={queryType}
            onChange={onQueryTypeChange}
          />
        </InlineField>
      </div>

      {showDatabasePicker && (
        <div className="gf-form">
          <InlineField
            label="Database"
            labelWidth={LABEL_WIDTH}
            tooltip="Select an Appwrite database. You can also type a database id manually."
            error={databasesError}
            invalid={!!databasesError}
            required
          >
            <Select<string>
              width={40}
              isClearable
              isLoading={loadingDatabases}
              options={databases}
              value={selectedDatabase}
              onChange={onDatabaseSelect}
              allowCustomValue
              placeholder="Select database"
              noOptionsMessage="No databases found"
            />
          </InlineField>
        </div>
      )}

      <div className="gf-form">
        <InlineField
          label="Collection"
          labelWidth={LABEL_WIDTH}
          tooltip="Select an Appwrite collection. The list is fetched for the selected database. You can also type a collection id manually."
          error={collectionsError}
          invalid={!!collectionsError}
          required
        >
          <Select<string>
            width={40}
            isClearable
            isLoading={loadingCollections}
            options={collections}
            value={selectedCollection}
            onChange={onCollectionSelect}
            allowCustomValue
            disabled={showDatabasePicker && !databaseId}
            placeholder={showDatabasePicker && !databaseId ? 'Select a database first' : 'Select collection'}
            noOptionsMessage="No collections found"
          />
        </InlineField>
      </div>

      {!isCount && (
        <div className="gf-form">
          <InlineField
            label="Attributes"
            labelWidth={LABEL_WIDTH}
            tooltip="Attributes (columns) to return, compiled into an Appwrite `select` query. Leave empty to return all attributes. Note: Appwrite always returns the system fields ($id, $permissions, $collectionId, ...) regardless of this selection — use 'Hide system fields' to drop them from the result."
            error={attributesError}
            invalid={!!attributesError}
          >
            <MultiSelect<string>
              width={40}
              isLoading={loadingAttributes}
              options={attributeOptions}
              value={selectedAttributes}
              onChange={onAttributesSelect}
              allowCustomValue
              placeholder={collectionId ? 'All attributes' : 'Select a collection first'}
              disabled={!collectionId}
              noOptionsMessage="No attributes found"
            />
          </InlineField>
          <InlineField
            label="Hide system fields"
            labelWidth={LABEL_WIDTH * 2}
            tooltip="Drop the Appwrite system fields ($id, $createdAt, $updatedAt, $collectionId, $databaseId, $permissions, $sequence) from the result, leaving only your collection's attributes."
          >
            <InlineSwitch value={!!hideSystemFields} onChange={onHideSystemFieldsChange} />
          </InlineField>
        </div>
      )}

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Filters"
          labelWidth={LABEL_WIDTH}
          tooltip="Filter documents. Add individual filters or nested filter groups. Compiled into Appwrite query strings server-side. Operators adapt to each attribute's type. Ignored when raw queries are set below."
        >
          <FilterEditor
            group={filterRoot}
            attributes={attributeList}
            disabled={!collectionId || usingRawQueries}
            onChange={onFilterChange}
          />
        </InlineField>
      </div>

      <div className="gf-form" style={{ alignItems: 'flex-start' }}>
        <InlineField
          label="Raw queries"
          labelWidth={LABEL_WIDTH}
          tooltip='Advanced. One Appwrite query string per line, for example: {"method":"equal","attribute":"status","values":["active"]}. When set, these take precedence over the structured filters above.'
        >
          <TextArea
            width={60}
            rows={3}
            value={rawQueries ?? ''}
            placeholder={'{"method":"equal","attribute":"status","values":["active"]}'}
            onChange={onRawQueriesChange}
            onBlur={onRunQuery}
            disabled={!collectionId}
          />
        </InlineField>
      </div>

      {!isCount && (
        <>
          <div className="gf-form" style={{ alignItems: 'flex-start' }}>
            <InlineField
              label="Sort"
              labelWidth={LABEL_WIDTH}
              tooltip="Order results by one or more attributes. Rows are applied in order. Attributes must be indexed in Appwrite to be sortable."
            >
              <div>
                {sortItems.map((item, index) => (
                  <InlineFieldRow key={index} style={{ marginBottom: 4, alignItems: 'center' }}>
                    <Select<string>
                      width={28}
                      options={attributeOptions}
                      value={
                        item.attribute
                          ? (attributeOptions.find((o) => o.value === item.attribute) ?? {
                              label: item.attribute,
                              value: item.attribute,
                            })
                          : null
                      }
                      onChange={(v) => onSortAttributeChange(index, v)}
                      allowCustomValue
                      placeholder="Select attribute"
                      disabled={!collectionId}
                      noOptionsMessage="No attributes found"
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
                <Button variant="secondary" size="sm" icon="plus" onClick={onAddSort} disabled={!collectionId}>
                  Add sort
                </Button>
              </div>
            </InlineField>
          </div>

          <div className="gf-form">
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of documents. 0 returns all (auto-paginated, 100 documents/request)."
            >
              <Input width={20} type="number" min={0} value={limit ?? 0} onChange={onLimitChange} onBlur={onRunQuery} />
            </InlineField>
          </div>
        </>
      )}
    </div>
  );
}
