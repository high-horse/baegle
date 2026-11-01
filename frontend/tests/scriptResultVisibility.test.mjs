import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createRequire} from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import {parse,compileScript} from '@vue/compiler-sfc';
import {createRenderer,defineComponent,h,nextTick,reactive,ref,watch} from 'vue';
const require=createRequire(import.meta.url);
function evaluate(source,overrides={}) {
    const output=ts.transpileModule(source,{compilerOptions:{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.CommonJS}}).outputText;
    const module={exports:{}};
    new Function('module','exports','require',output)(module,module.exports,name=>overrides[name]??require(name));
    return module.exports;
}
function component(path,overrides={}) {
    const {descriptor}=parse(readFileSync(new URL(path,import.meta.url),'utf8'));
    return evaluate(compileScript(descriptor,{id:path,inlineTemplate:true}).content,overrides).default;
}
const format=evaluate(readFileSync(new URL('../src/utils/formatDuration.ts',import.meta.url),'utf8'));
const GridData=defineComponent({props:['result'],setup:props=>()=>h('div',{class:'grid-data'},JSON.stringify(props.result.Rows))});
const ResultGrid=component('../src/components/workspace/resultGrid/ResultGrid.vue',{
    '@/utils/formatDuration':format,'./ResultTab.vue':{default:GridData},'./Schema/Index.vue':{default:{render:()=>null}},'../QueryElapsedTime.vue':{default:{render:()=>null}},
});
const ScriptResults=component('../src/components/workspace/ScriptResults.vue',{'@/utils/formatDuration':format,'./resultGrid/ResultGrid.vue':{default:ResultGrid}});
const renderer=createRenderer({
    createElement:type=>({type,props:{},children:[]}),createText:text=>({type:'#text',text,children:[]}),createComment:text=>({type:'#comment',text,children:[]}),
    insert(child,parent,anchor){if(child.parent){const old=child.parent.children.indexOf(child);if(old>=0)child.parent.children.splice(old,1);}const index=anchor?parent.children.indexOf(anchor):-1;parent.children.splice(index<0?parent.children.length:index,0,child);child.parent=parent;},
    remove(child){if(child.parent){const index=child.parent.children.indexOf(child);if(index>=0)child.parent.children.splice(index,1);child.parent=null;}},
    patchProp(node,key,_before,value){node.props[key]=value;},setText(node,text){node.text=text;},setElementText(node,text){node.text=text;node.children=[];},
    parentNode:node=>node.parent,nextSibling(node){if(!node.parent)return null;return node.parent.children[node.parent.children.indexOf(node)+1]??null;},
});
const generic=name=>defineComponent({inheritAttrs:false,setup:(_props,{attrs,slots})=>()=>h(name,attrs,slots.default?.())});
const Splitter=defineComponent({inheritAttrs:false,setup:(_props,{attrs,slots})=>()=>h('splitter',attrs,[h('before',slots.before?.()),h('after',slots.after?.())])});
function find(root,predicate){if(predicate(root))return root;for(const child of root.children??[]){const found=find(child,predicate);if(found)return found;}}
function setup(script=true) {
    const visible=ref(true),closed=[];
    const scriptResult={completed:2,total:2,duration:2,statements:[1,2].map(statement=>({statement,line:statement,preview:`SELECT ${statement}`,duration:1,results:[{columns:['value'],rows:[[statement]]}]}))};
    const tab=reactive({id:'console',type:'query',sql:'SELECT 1; SELECT 2;',loading:false,result:script?null:{Columns:[{Name:'value'}],Rows:[[1]],StartRow:1},scriptResult:script?scriptResult:undefined});
    const store={get activeTab(){return tab;},closeTab:id=>closed.push(id)};
    const Workspace=component('../src/components/workspace/WorkspaceTabContent.vue',{
        '@/stores/queryTabsStore':{useQueryTabsStore:()=>store},'@/stores/tableEditsStore':{useTableEditsStore:()=>({})},'@/utils/refreshTable':{refreshTable(){}},
        './QueryConsole.vue':{default:{render:()=>h('editor')}},'./resultGrid/ResultGrid.vue':{default:ResultGrid},'./ScriptResults.vue':{default:ScriptResults},
    });
    const app=renderer.createApp({render:()=>h(Workspace,{tab,showResults:visible.value,onHideResults:()=>visible.value=false})});
    app.component('q-splitter',Splitter);
    for(const tag of ['q-btn','q-select','q-tabs','q-tab','q-icon','q-spinner-dots'])app.component(tag,generic(tag));
    const root={type:'root',children:[]};app.mount(root);
    const layout=readFileSync(new URL('../src/layouts/WorkspaceLayout.vue',import.meta.url),'utf8').split('<script setup lang="ts">')[1].split('</script>')[0];
    const ast=ts.createSourceFile('layout.ts',layout,ts.ScriptTarget.Latest,true);
    const toggle=ast.statements.find(node=>ts.isFunctionDeclaration(node)&&node.name?.text==='handleToggleResultTabShortcut').getText(ast);
    const toggleSource=ts.transpileModule(toggle,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText;
    const press=new Function('queryTabsStore','showResults',`${toggleSource};return handleToggleResultTabShortcut;`)(store,visible);
    const runningWatch=ast.statements.find(node=>ts.isExpressionStatement(node)&&node.getText(ast).startsWith('watch(() => queryTabsStore.activeTab?.runningQuery')).getText(ast);
    const stop=new Function('watch','queryTabsStore','showResults',`return ${ts.transpileModule(runningWatch,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText}`)(watch,store,visible);
    return {tab,root,visible,closed,press,dispose(){stop();app.unmount();},get splitter(){return find(root,node=>node.type==='splitter');},button(label){return find(root,node=>node.type==='q-btn'&&node.props['aria-label']===label);}};
}
test('Ctrl+J hides and restores completed script results while preserving the selected statement',async()=>{
    const state=setup();
    const select=find(state.root,node=>node.type==='q-select');select.props['onUpdate:modelValue'](2);await nextTick();
    const grid=find(state.root,node=>node.props?.class==='grid-data');assert.equal(grid.text,'[[2]]');
    state.press({ctrlKey:true,key:'j',preventDefault(){}});await nextTick();
    assert.match(state.splitter.props.class,/results-collapsed/);assert.equal(state.splitter.props.disable,true);
    state.press({ctrlKey:true,key:'j',preventDefault(){}});await nextTick();
    assert.doesNotMatch(state.splitter.props.class,/results-collapsed/);
    assert.equal(find(state.root,node=>node.props?.class==='grid-data'),grid);
    assert.equal(grid.text,'[[2]]');state.dispose();
});
test('script close button hides the pane and new script execution reveals it again',async()=>{
    const state=setup();state.button('Hide script results').props.onClick();await nextTick();
    assert.equal(state.visible.value,false);assert.match(state.splitter.props.class,/results-collapsed/);
    state.tab.runningScript=true;state.tab.runningQuery=true;await nextTick();assert.equal(state.visible.value,true);
    state.press({ctrlKey:true,key:'j',preventDefault(){}});await nextTick();
    assert.match(state.splitter.props.class,/results-collapsed/);
    state.tab.runningScript=false;state.tab.runningQuery=false;await nextTick();
    assert.match(state.splitter.props.class,/results-collapsed/);state.dispose();
});
test('normal result close button hides the pane without closing the query console',async()=>{
    const state=setup(false);state.button('Hide results').props.onClick();await nextTick();
    assert.equal(state.visible.value,false);assert.deepEqual(state.closed,[]);state.dispose();
});
test('table result close button still closes its table tab',async()=>{
    const state=setup(false);state.tab.type='result';await nextTick();
    state.button('Close table tab').props.onClick();assert.deepEqual(state.closed,['console']);state.dispose();
});
